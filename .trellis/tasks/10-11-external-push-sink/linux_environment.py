"""Extract real TDLib libraries from the project's public runtime image."""
import io
import json
from pathlib import Path
import tarfile
import urllib.request

root = Path(__file__).resolve().parent / "linux-deps"
root.mkdir(exist_ok=True)
token = json.load(urllib.request.urlopen(
    "https://ghcr.io/token?scope=repository:killbus/budva:pull"))["token"]

def request(path, accept=None):
    headers = {"Authorization": "Bearer " + token}
    if accept:
        headers["Accept"] = accept
    return urllib.request.urlopen(urllib.request.Request(
        "https://ghcr.io/v2/killbus/budva/" + path, headers=headers))

accept = "application/vnd.oci.image.index.v1+json,application/vnd.oci.image.manifest.v1+json"
index = json.load(request("manifests/v0.2.0", accept))
digest = next(m["digest"] for m in index["manifests"]
              if m.get("platform", {}).get("architecture") == "amd64")
manifest = json.load(request("manifests/" + digest, accept))
for layer in manifest["layers"]:
    # Stream layers; write only TDLib files inside the isolated dependency root.
    with request("blobs/" + layer["digest"]) as response:
        with tarfile.open(fileobj=response, mode="r|gz") as archive:
            for item in archive:
                name = item.name.removeprefix("./")
                if not item.isfile() or not name.startswith("usr/local/lib/libtd"):
                    continue
                destination = root / Path(name).name
                with archive.extractfile(item) as source, destination.open("wb") as target:
                    while chunk := source.read(1024 * 1024):
                        target.write(chunk)
                print(destination.name, item.size, flush=True)
with urllib.request.urlopen(
    "https://raw.githubusercontent.com/tdlib/td/c15d3f5a5de6e3ba5839822c451152e5e18bb700/td/telegram/td_json_client.h"
) as response:
    header = root / "include/td/telegram/td_json_client.h"
    header.parent.mkdir(parents=True, exist_ok=True)
    header.write_bytes(response.read())
print("Real TDLib dependency extraction complete", flush=True)
