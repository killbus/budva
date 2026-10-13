package domain

import (
	"errors"
	"fmt"
)

// ErrInvalidSchemaVersion means the push target requested a webhook schema
// other than the supported 1 (v1) and 2 (v2).
var ErrInvalidSchemaVersion = errors.New("invalid schemaVersion: only 1 (v1) or 2 (v2) are supported")

// RuleSet описывает полную конфигурацию правил пересылки.
type RuleSet struct {
	// Sources — настройки источников по идентификатору чата.
	Sources map[ChatID]*Source `yaml:"sources"`
	// Destinations — настройки получателей по идентификатору чата.
	Destinations map[ChatID]*Destination `yaml:"destinations"`
	// PushTargets — внешние webhook-получатели по имени.
	PushTargets map[string]*PushTarget `yaml:"pushTargets"`
	// ForwardRules — правила пересылки по идентификатору правила.
	ForwardRules map[ForwardRuleID]*ForwardRule `yaml:"forwardRules"`
	// UniqueSources — множество уникальных идентификаторов источников.
	UniqueSources map[ChatID]struct{} `yaml:"-"`
	// UniqueDestinations — множество уникальных идентификаторов получателей.
	UniqueDestinations map[ChatID]struct{} `yaml:"-"`
	// OrderedForwardRules — идентификаторы правил в порядке обработки.
	OrderedForwardRules []ForwardRuleID `yaml:"-"`
}

// PushTarget описывает внешний HTTPS webhook.
type PushTarget struct {
	// URL — адрес webhook; допускается только схема https.
	URL string `yaml:"url"`
	// TimeoutSeconds — таймаут одной попытки доставки.
	TimeoutSeconds int `yaml:"timeoutSeconds"`
	// Headers — статические HTTP-заголовки запроса.
	Headers map[string]string `yaml:"headers"`
	// HMACSecret — необязательный секрет подписи тела запроса.
	HMACSecret string `yaml:"hmacSecret"`
	// SchemaVersion — версия webhook-схемы цели: пустое значение
	// (0) выбирает v1 по умолчанию, явные 1 и 2 выбираются как задано,
	// любое другое значение отклоняется при загрузке правила.
	SchemaVersion int `yaml:"schemaVersion"`
}

// EffectiveSchemaVersion resolves the webhook schema of the target: an
// absent value (0) selects v1 by default, explicit 1 and 2 select the
// requested schema, and any other value is rejected. The returned error
// wraps ErrInvalidSchemaVersion and carries no raw configuration data.
func (t *PushTarget) EffectiveSchemaVersion() (int, error) {
	switch t.SchemaVersion {
	case 0:
		return 1, nil
	case 1, 2:
		return t.SchemaVersion, nil
	default:
		return 0, fmt.Errorf("%w: requested %d", ErrInvalidSchemaVersion, t.SchemaVersion)
	}
}
