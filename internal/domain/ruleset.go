package domain

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
}
