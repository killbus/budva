package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"os/signal"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/pure-golang/budva-claude/internal/transport/grpc/pb"
)

const maxChunkSize = 256 * 1024

type options struct {
	address        string
	output         string
	fileID         int64
	chatID         int64
	messageID      int64
	timeout        time.Duration
	expectedSHA256 string
}

func main() {
	settings := options{}
	flag.StringVar(&settings.address, "addr", "localhost:50051", "facade plaintext gRPC host:port")
	flag.StringVar(&settings.output, "out", "", "destination file (must not exist)")
	flag.Int64Var(&settings.fileID, "file-id", 0, "TDLib file ID; excludes chat-id/message-id")
	flag.Int64Var(&settings.chatID, "chat-id", 0, "Telegram chat ID; requires message-id")
	flag.Int64Var(&settings.messageID, "message-id", 0, "Telegram message ID; requires chat-id")
	flag.DurationVar(&settings.timeout, "timeout", 10*time.Minute, "overall download deadline")
	flag.StringVar(&settings.expectedSHA256, "expected-sha256", "", "optional independently known SHA256")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected positional arguments")
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	err := run(ctx, settings)
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, "download failed:", err)
		os.Exit(1)
	}
}

func run(parent context.Context, settings options) error {
	if settings.address == "" || settings.output == "" || settings.timeout <= 0 {
		return errors.New("addr and out are required; timeout must be positive")
	}
	if settings.fileID != 0 {
		if settings.fileID < 1 || settings.fileID > math.MaxInt32 || settings.chatID != 0 || settings.messageID != 0 {
			return errors.New("file-id must be in [1, MaxInt32] and cannot be combined with chat-id/message-id")
		}
	} else if settings.chatID == 0 || settings.messageID <= 0 {
		return errors.New("specify file-id or both nonzero chat-id and positive message-id")
	}
	if settings.expectedSHA256 != "" {
		expected, err := hex.DecodeString(settings.expectedSHA256)
		if err != nil || len(expected) != sha256.Size {
			return errors.New("expected-sha256 must contain 64 hex digits")
		}
	}
	if _, err := os.Lstat(settings.output); !errors.Is(err, os.ErrNotExist) {
		if err == nil {
			return fmt.Errorf("destination already exists: %s", settings.output)
		}
		return err
	}
	partialPath := settings.output + ".part"
	partial, err := os.OpenFile(partialPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer partial.Close()
	finished := false
	defer func() {
		if !finished {
			fmt.Fprintf(os.Stderr, "partial output retained: %s\n", partialPath)
		}
	}()

	ctx, cancel := context.WithTimeout(parent, settings.timeout)
	defer cancel()
	connection, err := grpc.NewClient(settings.address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer connection.Close()
	remote := pb.NewFacadeGRPCClient(connection)
	started := time.Now()
	var stream grpc.ServerStreamingClient[pb.FileDownloadEvent]
	if settings.fileID != 0 {
		stream, err = remote.DownloadFile(ctx, &pb.DownloadFileRequest{FileId: settings.fileID})
	} else {
		stream, err = remote.GetMessageFile(ctx, &pb.GetMessageFileRequest{ChatId: settings.chatID, MessageId: settings.messageID})
	}
	if err != nil {
		return err
	}

	digest := sha256.New()
	var received int64
	metadataSeen, completeSeen := false, false
	for {
		event, recvErr := stream.Recv()
		if errors.Is(recvErr, io.EOF) {
			if !metadataSeen || !completeSeen {
				return errors.New("stream ended without metadata and verified completion")
			}
			break
		}
		if recvErr != nil {
			return recvErr
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if event == nil || completeSeen {
			return errors.New("empty event or event after completion")
		}
		switch frame := event.Event.(type) {
		case *pb.FileDownloadEvent_Metadata:
			if frame == nil || frame.Metadata == nil || metadataSeen || frame.Metadata.FileId <= 0 || frame.Metadata.Size < 0 {
				return errors.New("invalid or duplicate metadata")
			}
			metadataSeen = true
			fmt.Printf("file_id=%d mime_type=%q size=%d size_known=%t\n", frame.Metadata.FileId,
				frame.Metadata.MimeType, frame.Metadata.Size, frame.Metadata.SizeKnown)
		case *pb.FileDownloadEvent_Chunk:
			if !metadataSeen || frame == nil || frame.Chunk == nil {
				return errors.New("chunk missing metadata or payload")
			}
			chunk := frame.Chunk
			if len(chunk.Data) == 0 || len(chunk.Data) > maxChunkSize || chunk.Offset != received || received > math.MaxInt64-int64(len(chunk.Data)) {
				return errors.New("empty, oversized or nonsequential chunk")
			}
			written, writeErr := partial.Write(chunk.Data)
			if writeErr != nil {
				return writeErr
			}
			if written != len(chunk.Data) {
				return io.ErrShortWrite
			}
			if received == 0 {
				fmt.Printf("first_payload_after=%s bytes_saved=%d upstream_complete_sample=%t\n",
					time.Since(started), written, chunk.UpstreamComplete)
				fmt.Println("upstream_complete_sample describes server state before reading; it does not prove state at client receipt")
			}
			_, _ = digest.Write(chunk.Data)
			received += int64(written)
		case *pb.FileDownloadEvent_Complete:
			if !metadataSeen || frame == nil || frame.Complete == nil || frame.Complete.Size != received {
				return errors.New("invalid completion or final byte count mismatch")
			}
			completeSeen = true
		default:
			return errors.New("unknown or empty stream event")
		}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	actualSHA256 := hex.EncodeToString(digest.Sum(nil))
	if settings.expectedSHA256 != "" && !strings.EqualFold(settings.expectedSHA256, actualSHA256) {
		return fmt.Errorf("SHA256 mismatch: received %s", actualSHA256)
	}
	if err := partial.Sync(); err != nil {
		return err
	}
	if err := partial.Close(); err != nil {
		return err
	}
	reservation, err := os.OpenFile(settings.output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if err := reservation.Close(); err != nil {
		_ = os.Remove(settings.output)
		return err
	}
	if err := os.Rename(partialPath, settings.output); err != nil {
		_ = os.Remove(settings.output)
		return err
	}
	finished = true
	fmt.Printf("saved=%s bytes=%d sha256=%s elapsed=%s\n", settings.output, received, actualSHA256, time.Since(started))
	return nil
}
