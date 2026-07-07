package docs

import (
	"sort"

	dbtypes "whatsapp-go-api/internal/database/types"
)

const (
	documentVersion  = "1.0.0"
	whatsmeowVersion = "v0.0.0-20260630180629-b572e5bcb92b"
)

// Document is the structured source used to generate the public webhook docs.
type Document struct {
	Version         string            `json:"version"`
	GeneratedFrom   GeneratedFrom     `json:"generatedFrom"`
	Configuration   Configuration     `json:"configuration"`
	Delivery        Delivery          `json:"delivery"`
	Headers         []Header          `json:"headers"`
	Envelope        EnvelopeDoc       `json:"envelope"`
	CommonTypes     []CommonTypeDoc   `json:"commonTypes"`
	Events          []EventDoc        `json:"events"`
	IgnoredEvents   []IgnoredEventDoc `json:"ignoredEvents"`
	Compatibility   []string          `json:"compatibility"`
	Security        []string          `json:"security"`
	ErrorHandling   []string          `json:"errorHandling"`
	Ordering        []string          `json:"orderingAndConsistency"`
	GlobalNotes     []string          `json:"globalNotes"`
	SupportedEvents []string          `json:"supportedEvents"`
}

type GeneratedFrom struct {
	ConstantsPackage string   `json:"constantsPackage"`
	Dispatcher       string   `json:"dispatcher"`
	Normalizers      []string `json:"normalizers"`
	WhatsmeowVersion string   `json:"whatsmeowVersion"`
}

type Configuration struct {
	InstanceEndpoint string   `json:"instanceEndpoint"`
	FindEndpoint     string   `json:"findEndpoint"`
	GlobalEnv        []string `json:"globalEnv"`
	InstanceFields   []Field  `json:"instanceFields"`
	Notes            []string `json:"notes"`
}

type Delivery struct {
	Method                string   `json:"method"`
	DefaultWorkers        int      `json:"defaultWorkers"`
	DefaultQueueSize      int      `json:"defaultQueueSize"`
	HTTPTimeout           string   `json:"httpTimeout"`
	SuccessStatus         string   `json:"successStatus"`
	Retry                 string   `json:"retry"`
	InstanceFiltering     string   `json:"instanceFiltering"`
	GlobalFiltering       string   `json:"globalFiltering"`
	QueueFullBehavior     string   `json:"queueFullBehavior"`
	ExternalAttributes    string   `json:"externalAttributes"`
	AllowedWebhookSchemes []string `json:"allowedWebhookSchemes"`
}

type Header struct {
	Name        string `json:"name"`
	Value       string `json:"value"`
	Description string `json:"description"`
}

type EnvelopeDoc struct {
	DataType string   `json:"dataType"`
	Fields   []Field  `json:"fields"`
	Notes    []string `json:"notes"`
}

type CommonTypeDoc struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Fields      []Field `json:"fields"`
}

type EventDoc struct {
	Name                    string         `json:"name"`
	Flag                    string         `json:"flag"`
	Description             string         `json:"description"`
	InternalEvents          []string       `json:"internalEvents"`
	Persistence             string         `json:"persistence"`
	DataType                string         `json:"dataType"`
	DataSchema              string         `json:"dataSchema"`
	DynamicFields           bool           `json:"dynamicFields"`
	Fields                  []Field        `json:"fields"`
	PossibleValues          []PossibleEnum `json:"possibleValues,omitempty"`
	Example                 map[string]any `json:"example"`
	Notes                   []string       `json:"notes,omitempty"`
	ImplementedIn           []string       `json:"implementedIn"`
	RequiresPersistenceFlag string         `json:"requiresPersistenceFlag,omitempty"`
}

type Field struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Nullable    bool     `json:"nullable"`
	Description string   `json:"description"`
	Values      []string `json:"values,omitempty"`
}

type PossibleEnum struct {
	Field  string   `json:"field"`
	Values []string `json:"values"`
}

type IgnoredEventDoc struct {
	Name        string `json:"name"`
	Status      string `json:"status"`
	Description string `json:"description"`
}

func Build() Document {
	events := []EventDoc{
		callUpsertDoc(),
		chatsDeleteDoc(),
		chatsUpdatedDoc(),
		connectionUpdateDoc(),
		contactsUpdateDoc(),
		contactsUpsertDoc(),
		groupsParticipantsUpdateDoc(),
		groupsUpdateDoc(),
		groupsUpsertDoc(),
		historySyncDoc(),
		identityUpdateDoc(),
		labelsAssociationDoc(),
		labelsEditDoc(),
		mediaRetryDoc(),
		messagesDeleteDoc(),
		messagesStarDoc(),
		messagesUndecryptableDoc(),
		messagesUpdateDoc(),
		messagesUpsertDoc(),
		newsLetterDoc(),
		presenceUpdatedDoc(),
		profilePictureUpdateDoc(),
		qrcodeUpdatedDoc(),
		sendMessageDoc(),
		settingsUpdateDoc(),
		statusInstanceDoc(),
		userAboutUpdateDoc(),
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].Name < events[j].Name
	})

	return Document{
		Version: documentVersion,
		GeneratedFrom: GeneratedFrom{
			ConstantsPackage: "internal/database/types/webhook.go",
			Dispatcher:       "internal/webhook/manager.go",
			Normalizers: []string{
				"internal/webhook/payload.go",
				"internal/webhook/normalizer.go",
				"internal/whatsapp/service.go",
				"internal/whatsapp/event_persistence.go",
				"internal/whatsapp/webhook_events.go",
				"internal/whatsapp/webhook_extended_events.go",
				"internal/message/service.go",
				"internal/message/audio.go",
			},
			WhatsmeowVersion: whatsmeowVersion,
		},
		Configuration: Configuration{
			InstanceEndpoint: "PUT /webhook/set/{instanceName}",
			FindEndpoint:     "GET /webhook/find/{instanceName}",
			GlobalEnv: []string{
				"WEBHOOK_GLOBAL_URL",
				"WEBHOOK_GLOBAL_ENABLED",
			},
			InstanceFields: []Field{
				field("url", "string", true, false, "URL HTTP ou HTTPS usada para o webhook da instancia."),
				field("enabled", "boolean", true, false, "Habilita ou desabilita o webhook da instancia."),
				field("events", "object", false, false, "Mapa de flags por evento. Campo ausente preserva a configuracao atual; objeto vazio remove as flags."),
			},
			Notes: []string{
				"Campos desconhecidos em events sao rejeitados pela validacao do DTO/repository.",
				"A configuracao por instancia filtra eventos com base nas flags descritas neste documento.",
				"O webhook global, quando habilitado, recebe todos os eventos suportados sem consultar as flags da instancia.",
				"As URLs aceitas usam somente os esquemas http e https.",
			},
		},
		Delivery: Delivery{
			Method:                "POST",
			DefaultWorkers:        10,
			DefaultQueueSize:      1000,
			HTTPTimeout:           "15s",
			SuccessStatus:         "HTTP 2xx",
			Retry:                 "Sem retry automatico.",
			InstanceFiltering:     "O webhook da instancia entrega somente eventos habilitados em Webhook.events.",
			GlobalFiltering:       "O webhook global entrega todos os eventos suportados quando WEBHOOK_GLOBAL_ENABLED=true.",
			QueueFullBehavior:     "Fila cheia gera ErrWebhookQueueFull; o processamento principal registra warning e continua.",
			ExternalAttributes:    "instance.externalAttributes sempre e serializado como objeto; valores ausentes, null ou invalidos viram {}.",
			AllowedWebhookSchemes: []string{"http", "https"},
		},
		Headers: []Header{
			{Name: "Content-Type", Value: "application/json", Description: "Formato do payload."},
			{Name: "User-Agent", Value: "CodeChat-Webhook/1.0", Description: "Identifica o emissor do webhook."},
			{Name: "x-request-id", Value: "UUID ou request id do contexto", Description: "Id de rastreio da entrega; nao e uma chave de idempotencia garantida."},
			{Name: "x-owner-jid", Value: "JID do owner ou string vazia", Description: "Owner da instancia quando disponivel."},
			{Name: "x-instance-name", Value: "Nome da instancia", Description: "Nome publico da instancia."},
			{Name: "x-instance-id", Value: "1", Description: "Identificador numerico interno da instancia."},
			{Name: "x-webhook-event", Value: "Nome externo do evento", Description: "Mesmo valor do campo event no envelope."},
		},
		Envelope: EnvelopeDoc{
			DataType: "object",
			Fields: []Field{
				field("event", "string", true, false, "Nome externo do evento."),
				field("instance", "WebhookInstance", true, false, "Resumo da instancia que originou o evento."),
				field("data", "object | array", true, false, "Payload especifico do evento."),
				field("timestamp", "string", true, false, "Timestamp RFC3339 gerado no momento da montagem do envelope."),
			},
			Notes: []string{
				"Todos os exemplos abaixo mostram o envelope completo enviado ao endpoint.",
				"O campo data muda por evento, mas os campos de envelope permanecem estaveis.",
			},
		},
		CommonTypes: []CommonTypeDoc{
			{
				Name:        "WebhookInstance",
				Description: "Resumo da instancia usado em todos os envelopes.",
				Fields: []Field{
					field("id", "number", true, false, "Identificador numerico interno da instancia."),
					field("name", "string", true, false, "Nome da instancia."),
					field("connectionStatus", "string", true, false, "Status atual da conexao salvo na instancia, normalmente em lower case."),
					field("ownerJid", "string", true, true, "JID do owner da instancia; null quando nao existir."),
					field("externalAttributes", "object", true, false, "Atributos externos da instancia; sempre objeto."),
				},
			},
			{
				Name:        "MessageWebhookData",
				Description: "DTO usado por messages.upsert e send.message.",
				Fields:      messageFields(),
			},
			{
				Name:        "ContactUpsertWebhookData",
				Description: "DTO de criacao/atualizacao inicial de contato.",
				Fields:      contactUpsertFields(),
			},
			{
				Name:        "ContactUpdateWebhookData",
				Description: "DTO de atualizacao parcial de contato.",
				Fields:      contactUpdateFields(),
			},
			{
				Name:        "GroupParticipantWebhookData",
				Description: "Participante usado nos eventos de grupo.",
				Fields:      groupParticipantFields(),
			},
		},
		Events: events,
		IgnoredEvents: []IgnoredEventDoc{
			{Name: "PairPasskeyConfirmation", Status: "intentionally_ignored", Description: "Evento interativo de pareamento com codigo; nao e contrato de webhook."},
			{Name: "PairPasskeyError", Status: "handled_without_webhook", Description: "Tratado como estado/log de pareamento; nao ha payload publico estavel."},
			{Name: "PairPasskeyRequest", Status: "intentionally_ignored", Description: "Contem desafio/chave publica de pareamento e nao e serializado para webhook."},
			{Name: "QRScannedWithoutMultidevice", Status: "handled_without_webhook", Description: "O canal de QR converte o caso em falha de pareamento; emissoes diretas futuras caem em log fallback."},
			{Name: "MediaRetryError", Status: "internal_only", Description: "Struct auxiliar de erro usada dentro do payload de media.retry."},
			{Name: "MexNotificationData", Status: "internal_only", Description: "Struct auxiliar para notificacoes MEX sem evento publico dedicado."},
			{Name: "NewsletterMessageMeta", Status: "internal_only", Description: "Struct auxiliar usada dentro de news.letter."},
		},
		Compatibility: []string{
			"Os nomes oficiais de eventos sao os valores listados em events[].name.",
			"Campos booleanos de configuracao por instancia usam os nomes listados em events[].flag.",
			"Novos eventos do whatsmeow devem ser adicionados primeiro aos tipos oficiais em internal/database/types/webhook.go e depois a este contrato.",
		},
		Security: []string{
			"Nao ha assinatura HMAC, header de autenticacao ou segredo compartilhado no emissor atual.",
			"Use HTTPS, endpoints privados, allowlist de IP ou autenticacao no destino quando houver dados sensiveis.",
			"O x-request-id serve para rastreio e correlacao; ele nao prova autenticidade e nao garante idempotencia.",
		},
		ErrorHandling: []string{
			"Somente respostas HTTP 2xx sao consideradas sucesso.",
			"Falhas de rede, timeout e respostas nao 2xx sao registradas em log e nao abortam o fluxo principal.",
			"Nao existe retry automatico nem dead-letter queue nesta implementacao.",
		},
		Ordering: []string{
			"A fila de entrega e assincrona e processada por multiplos workers.",
			"A ordem relativa entre eventos nao e garantida entre instancias nem entre eventos diferentes da mesma instancia.",
			"Alguns eventos dependem de persistencia previa. Quando a persistencia falha, o evento pode nao ser emitido.",
		},
		GlobalNotes: []string{
			"Este documento descreve somente eventos realmente implementados no codigo atual.",
			"Os exemplos usam valores ilustrativos, mantendo o formato real do envelope e dos DTOs.",
			"Campos marcados como dinamicos podem receber propriedades adicionais conforme o evento de origem do whatsmeow.",
		},
		SupportedEvents: supportedEventNames(),
	}
}

func qrcodeUpdatedDoc() EventDoc {
	return EventDoc{
		Name:           string(dbtypes.WebhookEventQRCodeUpdated),
		Flag:           "qrcodeUpdated",
		Description:    "Novo QR Code disponivel para pareamento da instancia.",
		InternalEvents: []string{"QR channel"},
		Persistence:    "Atualiza o status da instancia para qr_code antes da entrega.",
		DataType:       "object",
		DataSchema:     "QRCodeUpdatedWebhookData",
		Fields: []Field{
			field("count", "number", true, false, "Quantidade de QRs emitidos nesta tentativa."),
			field("code", "string", true, false, "Codigo bruto do QR Code."),
			field("base64", "string", true, false, "Imagem do QR Code em data URL base64."),
			field("expiresInSeconds", "number", true, false, "Tempo restante informado pelo canal de QR, em segundos."),
			field("expiresAt", "string", true, false, "Timestamp RFC3339 UTC calculado para expiracao do QR Code."),
		},
		Example: envelopeWithInstance(dbtypes.WebhookEventQRCodeUpdated, map[string]any{
			"id":                 1,
			"name":               "codechat",
			"connectionStatus":   "qr_code",
			"ownerJid":           nil,
			"externalAttributes": map[string]any{},
		}, map[string]any{
			"count":            1,
			"code":             "2@abc",
			"base64":           "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAA",
			"expiresInSeconds": 60,
			"expiresAt":        "2026-07-04T18:01:00Z",
		}),
		Notes: []string{"Evento emitido pelo fluxo de QR, nao diretamente por um struct de evento do whatsmeow."},
		ImplementedIn: []string{
			"internal/whatsapp/service.go",
			"internal/webhook/manager.go",
		},
	}
}

func historySyncDoc() EventDoc {
	return EventDoc{
		Name:           string(dbtypes.WebhookEventHistorySync),
		Flag:           "historySync",
		Description:    "Sincronizacao de historico recebida do WhatsApp.",
		InternalEvents: []string{"*events.HistorySync"},
		Persistence:    "Nao persiste dados especificos antes da entrega do webhook.",
		DataType:       "object",
		DataSchema:     "HistorySyncWebhookData",
		DynamicFields:  true,
		Fields: []Field{
			field("type", "string", true, false, "Tipo fixo do payload.", "history.sync"),
			field("dateTime", "string", true, false, "Timestamp RFC3339 UTC do evento ou do processamento."),
			field("data", "object", false, false, "Conteudo normalizado do evento de historico quando disponivel."),
		},
		PossibleValues: []PossibleEnum{{Field: "type", Values: []string{"history.sync"}}},
		Example: envelope(dbtypes.WebhookEventHistorySync, map[string]any{
			"type":     "history.sync",
			"dateTime": "2026-07-04T18:00:00Z",
			"data": map[string]any{
				"syncType": "INITIAL_BOOTSTRAP",
			},
		}),
		Notes: []string{"Payload dinamico porque o conteudo vem do proto de historico do whatsmeow."},
		ImplementedIn: []string{
			"internal/whatsapp/service.go",
			"internal/whatsapp/webhook_extended_events.go",
		},
	}
}

func messagesUpsertDoc() EventDoc {
	return EventDoc{
		Name:                    string(dbtypes.WebhookEventMessagesUpsert),
		Flag:                    "messagesUpsert",
		Description:             "Mensagem recebida e persistida pela aplicacao.",
		InternalEvents:          []string{"*events.Message", "*events.FBMessage"},
		Persistence:             "Requer DATABASE_SAVE_DATA_NEW_MESSAGE=true. A mensagem e persistida com CreateOrIgnore e relida antes da entrega.",
		RequiresPersistenceFlag: "DATABASE_SAVE_DATA_NEW_MESSAGE",
		DataType:                "object",
		DataSchema:              "MessageWebhookData",
		Fields:                  messageFields(),
		Example: envelope(dbtypes.WebhookEventMessagesUpsert, map[string]any{
			"id":                1024,
			"keyRemoteJid":      "5511999999999@s.whatsapp.net",
			"keyLid":            nil,
			"keyFromMe":         false,
			"keyParticipant":    nil,
			"keyParticipantLid": nil,
			"pushName":          "Cliente",
			"messageType":       "conversation",
			"content":           map[string]any{"text": "Ola"},
			"messageTimestamp":  1783188000,
			"device":            "ios",
			"isGroup":           false,
			"metadata":          nil,
		}),
		Notes: []string{"Se a persistencia ou a releitura da mensagem falhar, o webhook nao e emitido."},
		ImplementedIn: []string{
			"internal/whatsapp/service.go",
			"internal/whatsapp/event_persistence.go",
			"internal/webhook/payload.go",
		},
	}
}

func messagesUpdateDoc() EventDoc {
	return EventDoc{
		Name:                    string(dbtypes.WebhookEventMessagesUpdated),
		Flag:                    "messagesUpdated",
		Description:             "Atualizacao de recibo/status de uma mensagem ja conhecida.",
		InternalEvents:          []string{"*events.Receipt"},
		Persistence:             "Requer DATABASE_SAVE_MESSAGE_UPDATE=true. O handler tenta localizar a mensagem, persiste a atualizacao e so entao entrega.",
		RequiresPersistenceFlag: "DATABASE_SAVE_MESSAGE_UPDATE",
		DataType:                "object",
		DataSchema:              "MessageUpdateWebhookData",
		Fields: []Field{
			field("messageId", "number", true, false, "ID interno da mensagem persistida."),
			field("status", "string", true, false, "Status normalizado do recibo.", "delivered", "sent", "read", "played", "server_error", "retry", "unknown"),
			field("dateTime", "string", true, false, "Timestamp RFC3339 UTC do recibo; usa o timestamp do evento ou o horario do processamento."),
		},
		PossibleValues: []PossibleEnum{{Field: "status", Values: []string{"delivered", "sent", "read", "played", "server_error", "retry", "unknown"}}},
		Example: envelope(dbtypes.WebhookEventMessagesUpdated, map[string]any{
			"messageId": 1024,
			"status":    "read",
			"dateTime":  "2026-07-04T18:05:00Z",
		}),
		Notes: []string{"Quando a mensagem referenciada nao e encontrada, o evento e descartado apos as tentativas configuradas no handler."},
		ImplementedIn: []string{
			"internal/whatsapp/service.go",
			"internal/whatsapp/event_persistence.go",
			"internal/webhook/payload.go",
		},
	}
}

func messagesDeleteDoc() EventDoc {
	return EventDoc{
		Name:           string(dbtypes.WebhookEventMessagesDeleted),
		Flag:           "messagesDeleted",
		Description:    "Mensagem removida localmente pelo evento DeleteForMe.",
		InternalEvents: []string{"*events.DeleteForMe"},
		Persistence:    "Nao persiste dados especificos antes da entrega do webhook.",
		DataType:       "object",
		DataSchema:     "MessageDeletedWebhookData",
		Fields: []Field{
			field("chatJid", "string", true, false, "JID da conversa."),
			field("senderJid", "string", false, false, "JID do remetente quando disponivel; omitido quando ausente."),
			field("keyFromMe", "boolean", true, false, "Indica se a mensagem era da propria instancia."),
			field("messageId", "string", true, false, "ID da mensagem apagada."),
			field("deleteMedia", "boolean", true, false, "Indica se a midia local deve ser removida."),
			field("fromFullSync", "boolean", true, false, "Indica se veio de sincronizacao completa."),
			field("dateTime", "string", true, false, "Timestamp RFC3339 UTC do evento ou do processamento."),
			field("messageTime", "string", false, false, "Timestamp RFC3339 UTC original da mensagem quando informado; omitido quando ausente."),
		},
		Example: envelope(dbtypes.WebhookEventMessagesDeleted, map[string]any{
			"chatJid":      "5511999999999@s.whatsapp.net",
			"senderJid":    "5511988888888@s.whatsapp.net",
			"keyFromMe":    false,
			"messageId":    "ABC123",
			"deleteMedia":  true,
			"fromFullSync": false,
			"dateTime":     "2026-07-04T18:00:00Z",
			"messageTime":  "2026-07-04T17:59:00Z",
		}),
		ImplementedIn: []string{
			"internal/whatsapp/service.go",
			"internal/whatsapp/webhook_events.go",
		},
	}
}

func messagesStarDoc() EventDoc {
	return EventDoc{
		Name:           string(dbtypes.WebhookEventMessagesStarred),
		Flag:           "messagesStarred",
		Description:    "Marcacao ou desmarcacao de estrela em mensagem.",
		InternalEvents: []string{"*events.Star"},
		Persistence:    "Nao persiste dados especificos antes da entrega do webhook.",
		DataType:       "object",
		DataSchema:     "MessageStarredWebhookData",
		Fields: []Field{
			field("chatJid", "string", true, false, "JID da conversa."),
			field("senderJid", "string", false, false, "JID do remetente quando disponivel; omitido quando ausente."),
			field("keyFromMe", "boolean", true, false, "Indica se a mensagem e da propria instancia."),
			field("messageId", "string", true, false, "ID da mensagem."),
			field("starred", "boolean", true, false, "true quando marcada com estrela."),
			field("fromFullSync", "boolean", true, false, "Indica se veio de sincronizacao completa."),
			field("dateTime", "string", true, false, "Timestamp RFC3339 UTC do evento ou do processamento."),
		},
		Example: envelope(dbtypes.WebhookEventMessagesStarred, map[string]any{
			"chatJid":      "5511999999999@s.whatsapp.net",
			"senderJid":    "5511988888888@s.whatsapp.net",
			"keyFromMe":    false,
			"messageId":    "ABC123",
			"starred":      true,
			"fromFullSync": false,
			"dateTime":     "2026-07-04T18:00:00Z",
		}),
		ImplementedIn: []string{
			"internal/whatsapp/service.go",
			"internal/whatsapp/webhook_events.go",
		},
	}
}

func messagesUndecryptableDoc() EventDoc {
	return EventDoc{
		Name:           string(dbtypes.WebhookEventMessagesUndecryptable),
		Flag:           "messagesUndecryptable",
		Description:    "Mensagem recebida sem possibilidade de descriptografia.",
		InternalEvents: []string{"*events.UndecryptableMessage"},
		Persistence:    "Nao persiste dados especificos antes da entrega do webhook.",
		DataType:       "object",
		DataSchema:     "MessageUndecryptableWebhookData",
		Fields: []Field{
			field("keyId", "string", true, false, "ID da chave/mensagem que falhou."),
			field("chatJid", "string", true, false, "JID da conversa."),
			field("senderJid", "string", false, false, "JID do remetente quando disponivel; omitido quando ausente."),
			field("keyFromMe", "boolean", true, false, "Indica se a mensagem e da propria instancia."),
			field("isUnavailable", "boolean", true, false, "Indica se o conteudo foi marcado como indisponivel."),
			field("unavailableType", "string", false, false, "Tipo de indisponibilidade.", "view_once"),
			field("decryptFailMode", "string", false, false, "Modo de falha reportado.", "hide"),
			field("dateTime", "string", true, false, "Timestamp RFC3339 UTC do evento ou do processamento."),
		},
		PossibleValues: []PossibleEnum{
			{Field: "unavailableType", Values: []string{"view_once"}},
			{Field: "decryptFailMode", Values: []string{"hide"}},
		},
		Example: envelope(dbtypes.WebhookEventMessagesUndecryptable, map[string]any{
			"keyId":           "ABC123",
			"chatJid":         "5511999999999@s.whatsapp.net",
			"senderJid":       "5511988888888@s.whatsapp.net",
			"keyFromMe":       false,
			"isUnavailable":   true,
			"unavailableType": "view_once",
			"decryptFailMode": "hide",
			"dateTime":        "2026-07-04T18:00:00Z",
		}),
		Notes: []string{"Campos vazios sao omitidos por omitempty."},
		ImplementedIn: []string{
			"internal/whatsapp/service.go",
			"internal/whatsapp/webhook_events.go",
		},
	}
}

func sendMessageDoc() EventDoc {
	return EventDoc{
		Name:           string(dbtypes.WebhookEventSendMessage),
		Flag:           "sendMessage",
		Description:    "Mensagem enviada pela API apos envio e persistencia bem-sucedidos.",
		InternalEvents: []string{"message service send result"},
		Persistence:    "Persistida antes da entrega pelo fluxo de envio de mensagens da API.",
		DataType:       "object",
		DataSchema:     "MessageWebhookData",
		Fields:         messageFields(),
		Example: envelope(dbtypes.WebhookEventSendMessage, map[string]any{
			"id":                2048,
			"keyRemoteJid":      "5511999999999@s.whatsapp.net",
			"keyLid":            nil,
			"keyFromMe":         true,
			"keyParticipant":    nil,
			"keyParticipantLid": nil,
			"pushName":          nil,
			"messageType":       "conversation",
			"content":           map[string]any{"text": "Mensagem enviada"},
			"messageTimestamp":  1783188000,
			"device":            "web",
			"isGroup":           false,
			"metadata":          nil,
		}),
		Notes: []string{
			"Usa o mesmo DTO de messages.upsert, mas a origem e o envio pela propria API.",
			"Quando uma mensagem e aceita com options.mentionAll=true, o mesmo evento send.message tambem entrega o resultado definitivo do processamento assincrono. Nesse caso, data contem processId, status, mentionAll, externalAttributes e, em caso de sucesso, data.messageId, data.remoteJid, data.participantCount e data.timestamp. Em caso de falha, contem error.code e error.message.",
		},
		ImplementedIn: []string{
			"internal/message/service.go",
			"internal/message/audio.go",
			"internal/webhook/payload.go",
		},
	}
}

func contactsUpsertDoc() EventDoc {
	return EventDoc{
		Name:                    string(dbtypes.WebhookEventContactsUpsert),
		Flag:                    "contactsUpsert",
		Description:             "Contato criado ou atualizado no cadastro local.",
		InternalEvents:          []string{"*events.Contact"},
		Persistence:             "Requer DATABASE_SAVE_DATA_CONTACTS=true. O contato e salvo antes da entrega.",
		RequiresPersistenceFlag: "DATABASE_SAVE_DATA_CONTACTS",
		DataType:                "object",
		DataSchema:              "ContactUpsertWebhookData",
		Fields:                  contactUpsertFields(),
		PossibleValues:          []PossibleEnum{{Field: "action", Values: []string{"upserted"}}},
		Example: envelope(dbtypes.WebhookEventContactsUpsert, map[string]any{
			"id":            41,
			"remoteJid":     "5511999999999@s.whatsapp.net",
			"lid":           "279847268053216@lid",
			"pushName":      "Cliente",
			"profilePicUrl": nil,
			"action":        "upserted",
		}),
		ImplementedIn: []string{
			"internal/whatsapp/service.go",
			"internal/whatsapp/event_persistence.go",
		},
	}
}

func contactsUpdateDoc() EventDoc {
	return EventDoc{
		Name:                    string(dbtypes.WebhookEventContactsUpdated),
		Flag:                    "contactsUpdated",
		Description:             "Atualizacao parcial em contato existente.",
		InternalEvents:          []string{"*events.PushName", "*events.BusinessName"},
		Persistence:             "Requer DATABASE_SAVE_DATA_CONTACTS=true. O contato e atualizado antes da entrega quando aplicavel.",
		RequiresPersistenceFlag: "DATABASE_SAVE_DATA_CONTACTS",
		DataType:                "array",
		DataSchema:              "ContactUpdateWebhookData[]",
		Fields:                  contactUpdateFields(),
		PossibleValues: []PossibleEnum{
			{Field: "action", Values: []string{"updated"}},
			{Field: "source", Values: []string{"pushName", "businessName"}},
		},
		Example: envelope(dbtypes.WebhookEventContactsUpdated, []any{
			map[string]any{
				"id":           41,
				"remoteJid":    "5511999999999@s.whatsapp.net",
				"lid":          nil,
				"pushName":     "Cliente Atualizado",
				"businessName": nil,
				"action":       "updated",
				"source":       "pushName",
			},
		}),
		Notes: []string{"O payload e array; o handler atual normalmente envia um item por entrega."},
		ImplementedIn: []string{
			"internal/whatsapp/service.go",
			"internal/whatsapp/event_persistence.go",
			"internal/whatsapp/webhook_extended_events.go",
		},
	}
}

func chatsUpdatedDoc() EventDoc {
	return EventDoc{
		Name:           string(dbtypes.WebhookEventChatsUpdated),
		Flag:           "chatsUpdated",
		Description:    "Atualizacao de propriedades de conversas.",
		InternalEvents: []string{"*events.Blocklist", "*events.BlocklistChange", "*events.Archive", "*events.UnarchiveChatsSetting", "*events.ClearChat", "*events.Pin", "*events.Mute", "*events.MarkChatAsRead"},
		Persistence:    "Nao persiste dados especificos antes da entrega do webhook.",
		DataType:       "object",
		DataSchema:     "ChatUpdatedWebhookData",
		DynamicFields:  true,
		Fields: []Field{
			field("type", "string", true, false, "Subtipo da atualizacao de chat.", "blocklist", "blocklist.change", "archive", "unarchive.setting", "clear", "pin", "mute", "mark.read"),
			field("dateTime", "string", true, false, "Timestamp RFC3339 UTC do evento ou do processamento."),
			field("chatJid", "string", false, false, "JID da conversa quando o subtipo tem conversa especifica."),
			field("fromFullSync", "boolean", false, false, "Indica se veio de sincronizacao completa quando disponivel."),
			field("additionalProperties", "object", false, false, "Campos achatados do evento original do whatsmeow."),
		},
		PossibleValues: []PossibleEnum{{Field: "type", Values: []string{"blocklist", "blocklist.change", "archive", "unarchive.setting", "clear", "pin", "mute", "mark.read"}}},
		Example: envelope(dbtypes.WebhookEventChatsUpdated, map[string]any{
			"type":     "archive",
			"dateTime": "2026-07-04T18:00:00Z",
			"chatJid":  "5511999999999@s.whatsapp.net",
			"archived": true,
		}),
		Notes: []string{"Eventos UserStatusMute sao documentados em settings.update, porque o registro atual os roteia para esse evento externo."},
		ImplementedIn: []string{
			"internal/whatsapp/service.go",
			"internal/whatsapp/webhook_events.go",
		},
	}
}

func chatsDeleteDoc() EventDoc {
	return EventDoc{
		Name:           string(dbtypes.WebhookEventChatsDeleted),
		Flag:           "chatsDeleted",
		Description:    "Exclusao ou limpeza de conversa.",
		InternalEvents: []string{"*events.DeleteChat"},
		Persistence:    "Nao persiste dados especificos antes da entrega do webhook.",
		DataType:       "object",
		DataSchema:     "ChatDeletedWebhookData",
		DynamicFields:  true,
		Fields: []Field{
			field("chatJid", "string", true, false, "JID da conversa."),
			field("dateTime", "string", true, false, "Timestamp RFC3339 do processamento."),
			field("deleteMedia", "boolean", false, false, "Indica remocao de midia local quando presente."),
			field("additionalProperties", "object", false, false, "Campos achatados da acao original."),
		},
		Example: envelope(dbtypes.WebhookEventChatsDeleted, map[string]any{
			"chatJid":     "5511999999999@s.whatsapp.net",
			"dateTime":    "2026-07-04T18:00:00Z",
			"deleteMedia": false,
		}),
		ImplementedIn: []string{
			"internal/whatsapp/service.go",
			"internal/whatsapp/webhook_events.go",
		},
	}
}

func presenceUpdatedDoc() EventDoc {
	return EventDoc{
		Name:           string(dbtypes.WebhookEventPresenceUpdated),
		Flag:           "presenceUpdated",
		Description:    "Atualizacao de presenca de usuario ou presenca em chat.",
		InternalEvents: []string{"*events.ChatPresence", "*events.Presence"},
		Persistence:    "Nao persiste dados especificos antes da entrega do webhook.",
		DataType:       "object",
		DataSchema:     "PresenceUpdatedWebhookData",
		DynamicFields:  true,
		Fields: []Field{
			field("type", "string", false, false, "Tipo fixo presence no payload vindo de *events.Presence.", "presence"),
			field("chatJid", "string", false, false, "JID do chat no payload de ChatPresence."),
			field("senderJid", "string", false, false, "JID do remetente no payload de ChatPresence."),
			field("state", "string", false, false, "Estado de presenca no payload de ChatPresence."),
			field("media", "string", false, false, "Tipo de midia quando presenca esta relacionada a midia."),
			field("jid", "string", false, false, "JID no payload de Presence."),
			field("unavailable", "boolean", false, false, "Indica indisponibilidade no payload de Presence."),
			field("lastSeen", "string", false, false, "Ultimo visto quando informado."),
			field("dateTime", "string", true, false, "Timestamp RFC3339 do processamento."),
		},
		PossibleValues: []PossibleEnum{{Field: "type", Values: []string{"presence"}}},
		Example: envelope(dbtypes.WebhookEventPresenceUpdated, map[string]any{
			"chatJid":   "5511999999999@s.whatsapp.net",
			"senderJid": "5511999999999@s.whatsapp.net",
			"state":     "composing",
			"media":     "text",
			"dateTime":  "2026-07-04T18:00:00Z",
		}),
		Notes: []string{"O formato varia entre ChatPresence e Presence; use os campos presentes no payload recebido."},
		ImplementedIn: []string{
			"internal/whatsapp/service.go",
			"internal/whatsapp/webhook_events.go",
		},
	}
}

func groupsUpsertDoc() EventDoc {
	return EventDoc{
		Name:           string(dbtypes.WebhookEventGroupsUpsert),
		Flag:           "groupsUpsert",
		Description:    "Grupo criado, descoberto ou sincronizado.",
		InternalEvents: []string{"*events.JoinedGroup"},
		Persistence:    "Nao persiste dados especificos antes da entrega do webhook.",
		DataType:       "array",
		DataSchema:     "GroupUpsertWebhookData[]",
		Fields:         groupUpsertFields(),
		Example: envelope(dbtypes.WebhookEventGroupsUpsert, []any{
			map[string]any{
				"id":             "120363000000000000@g.us",
				"addressingMode": "pn",
				"owner":          "5531999999999@s.whatsapp.net",
				"subject":        "Grupo",
				"isCommunity":    false,
				"participants": []any{
					map[string]any{
						"id":           "5511999999999@s.whatsapp.net",
						"lid":          "279847268053216@lid",
						"isAdmin":      true,
						"isSuperAdmin": false,
						"admin":        "admin",
					},
				},
				"creation": 1783187000,
			},
		}),
		Notes: []string{"O payload e array para compatibilidade com contratos de lista, mesmo quando uma entrega contem um grupo."},
		ImplementedIn: []string{
			"internal/whatsapp/service.go",
			"internal/whatsapp/webhook_extended_events.go",
		},
	}
}

func groupsUpdateDoc() EventDoc {
	return EventDoc{
		Name:           string(dbtypes.WebhookEventGroupsUpdated),
		Flag:           "groupsUpdated",
		Description:    "Atualizacao parcial de metadados de grupo.",
		InternalEvents: []string{"*events.GroupInfo"},
		Persistence:    "Nao persiste dados especificos antes da entrega do webhook.",
		DataType:       "array",
		DataSchema:     "GroupUpdateWebhookData[]",
		Fields:         groupUpdateFields(),
		Example: envelope(dbtypes.WebhookEventGroupsUpdated, []any{
			map[string]any{
				"partial": map[string]any{
					"id":          "120363000000000000@g.us",
					"subject":     "Novo nome do grupo",
					"announce":    true,
					"subjectTime": 1783188000,
				},
			},
		}),
		Notes: []string{"O handler atual envia array com um item contendo partial."},
		ImplementedIn: []string{
			"internal/whatsapp/service.go",
			"internal/whatsapp/webhook_extended_events.go",
		},
	}
}

func groupsParticipantsUpdateDoc() EventDoc {
	return EventDoc{
		Name:           string(dbtypes.WebhookEventGroupsParticipantsUpdated),
		Flag:           "groupsParticipantsUpdated",
		Description:    "Mudanca de participantes em grupo.",
		InternalEvents: []string{"*events.GroupInfo"},
		Persistence:    "Nao persiste dados especificos antes da entrega do webhook.",
		DataType:       "object",
		DataSchema:     "GroupParticipantsUpdatedWebhookData",
		Fields: []Field{
			field("id", "string", true, false, "JID do grupo."),
			field("author", "string", true, false, "JID do autor da alteracao; string vazia quando ausente."),
			field("authorPn", "string", false, false, "Phone number do autor quando disponivel; omitido quando ausente."),
			field("participants", "GroupParticipantWebhookData[]", true, false, "Participantes afetados."),
			field("action", "string", true, false, "Acao aplicada.", "add", "remove", "promote", "demote"),
		},
		PossibleValues: []PossibleEnum{{Field: "action", Values: []string{"add", "remove", "promote", "demote"}}},
		Example: envelope(dbtypes.WebhookEventGroupsParticipantsUpdated, map[string]any{
			"id":       "120363000000000000@g.us",
			"author":   "5531999999999@s.whatsapp.net",
			"authorPn": "5531999999999",
			"participants": []any{
				map[string]any{
					"id":           "5511999999999@s.whatsapp.net",
					"isAdmin":      false,
					"isSuperAdmin": false,
					"admin":        nil,
				},
			},
			"action": "add",
		}),
		ImplementedIn: []string{
			"internal/whatsapp/service.go",
			"internal/whatsapp/webhook_extended_events.go",
		},
	}
}

func connectionUpdateDoc() EventDoc {
	return EventDoc{
		Name:           string(dbtypes.WebhookEventConnectionUpdated),
		Flag:           "connectionUpdated",
		Description:    "Mudanca de estado da conexao da instancia.",
		InternalEvents: []string{"*events.PairSuccess", "*events.PairError", "*events.Connected", "*events.Disconnected", "*events.LoggedOut", "*events.StreamReplaced", "*events.KeepAliveTimeout", "*events.KeepAliveRestored", "*events.ConnectFailure", "*events.ManualLoginReconnect", "*events.StreamError", "*events.CATRefreshError"},
		Persistence:    "O status da instancia e atualizado pelos fluxos de conexao antes ou junto da entrega conforme o subtipo.",
		DataType:       "object",
		DataSchema:     "ConnectionUpdateWebhookData",
		Fields: []Field{
			field("type", "string", true, false, "Subtipo normalizado da conexao.", "pair.success", "connected", "disconnected", "logged.out", "stream.replaced", "keepalive.timeout", "keepalive.restored", "connect.failure", "manual.reconnect", "pair.error", "stream.error", "cat.refresh.error"),
			field("connection", "string", true, false, "Estado externo da conexao.", "connecting", "open", "close", "replaced", "timeout"),
			field("statusReason", "number", false, false, "Codigo numerico de motivo quando diferente de zero; omitido quando zero."),
			field("lastConnection", "string", false, false, "Timestamp RFC3339 UTC quando informado; omitido quando ausente."),
			field("message", "string", false, false, "Mensagem tecnica quando informada; omitida quando vazia."),
		},
		PossibleValues: []PossibleEnum{
			{Field: "type", Values: []string{"pair.success", "connected", "disconnected", "logged.out", "stream.replaced", "keepalive.timeout", "keepalive.restored", "connect.failure", "manual.reconnect", "pair.error", "stream.error", "cat.refresh.error"}},
			{Field: "connection", Values: []string{"connecting", "open", "close", "replaced", "timeout"}},
		},
		Example: envelope(dbtypes.WebhookEventConnectionUpdated, map[string]any{
			"type":           "connected",
			"connection":     "open",
			"lastConnection": "2026-07-04T18:50:00Z",
		}),
		Notes: []string{"`statusReason`, `lastConnection` e `message` usam `omitempty`; quando estao zerados ou vazios, nao aparecem no JSON."},
		ImplementedIn: []string{
			"internal/whatsapp/service.go",
			"internal/webhook/payload.go",
		},
	}
}

func statusInstanceDoc() EventDoc {
	return EventDoc{
		Name:           string(dbtypes.WebhookEventStatusInstance),
		Flag:           "statusInstance",
		Description:    "Eventos de estado operacional ou avisos da instancia.",
		InternalEvents: []string{"*events.ClientOutdated", "*events.TemporaryBan", "*events.OfflineSyncPreview", "*events.OfflineSyncCompleted", "*events.PrivacySettings", "*events.AppState", "*events.AppStateSyncComplete", "*events.AppStateSyncError", "*events.AccountReachoutTimelock"},
		Persistence:    "Nao persiste dados especificos antes da entrega do webhook.",
		DataType:       "object",
		DataSchema:     "InstanceStatusWebhookData",
		DynamicFields:  true,
		Fields: []Field{
			field("type", "string", true, false, "Subtipo do status da instancia."),
			field("status", "string", false, false, "Status textual do subtipo; omitido quando vazio."),
			field("message", "string", false, false, "Mensagem tecnica ou humana; omitida quando vazia."),
			field("data", "object", false, false, "Dados adicionais do subtipo; omitido quando ausente."),
		},
		PossibleValues: []PossibleEnum{{Field: "type", Values: []string{"client.outdated", "temporary.ban", "offline.sync.preview", "offline.sync.completed", "privacy.settings", "app.state", "app.state.sync.completed", "app.state.sync.error", "account.reachout.timelock"}}},
		Example: envelope(dbtypes.WebhookEventStatusInstance, map[string]any{
			"type":   "offline.sync.completed",
			"status": "completed",
			"data": map[string]any{
				"count": 185,
			},
		}),
		ImplementedIn: []string{
			"internal/whatsapp/service.go",
			"internal/webhook/payload.go",
		},
	}
}

func newsLetterDoc() EventDoc {
	return EventDoc{
		Name:           string(dbtypes.WebhookEventNewsletter),
		Flag:           "newsLetter",
		Description:    "Eventos relacionados a newsletters/canais.",
		InternalEvents: []string{"*events.NewsletterJoin", "*events.NewsletterLeave", "*events.NewsletterLiveUpdate", "*events.NewsletterMessageMeta", "*events.NewsletterMuteChange"},
		Persistence:    "Nao persiste dados especificos antes da entrega do webhook.",
		DataType:       "object",
		DataSchema:     "NewsLetterWebhookData",
		DynamicFields:  true,
		Fields: []Field{
			field("type", "string", true, false, "Subtipo do evento de newsletter.", "join", "leave", "live.update", "message.meta", "mute.change"),
			field("newsletterJid", "string", false, false, "JID da newsletter quando a origem traz id ou jid."),
			field("dateTime", "string", true, false, "Timestamp RFC3339 do processamento."),
			field("additionalProperties", "object", false, false, "Campos achatados do evento original."),
		},
		PossibleValues: []PossibleEnum{{Field: "type", Values: []string{"join", "leave", "live.update", "message.meta", "mute.change"}}},
		Example: envelope(dbtypes.WebhookEventNewsletter, map[string]any{
			"type":          "mute.change",
			"newsletterJid": "120363000000000000@newsletter",
			"muted":         true,
			"dateTime":      "2026-07-04T18:00:00Z",
		}),
		ImplementedIn: []string{
			"internal/whatsapp/service.go",
			"internal/whatsapp/webhook_extended_events.go",
		},
	}
}

func callUpsertDoc() EventDoc {
	return EventDoc{
		Name:           string(dbtypes.WebhookEventCallUpsert),
		Flag:           "callUpsert",
		Description:    "Atualizacao de chamada de voz ou video.",
		InternalEvents: []string{"*events.CallOffer", "*events.CallAccept", "*events.CallOfferNotice", "*events.CallPreAccept", "*events.CallTransport", "*events.CallTerminate", "*events.CallReject", "*events.CallRelayLatency", "*events.UnknownCallEvent"},
		Persistence:    "Nao persiste dados especificos antes da entrega do webhook.",
		DataType:       "object",
		DataSchema:     "CallUpsertWebhookData",
		Fields: []Field{
			field("chatId", "string", true, false, "JID do chat da chamada."),
			field("from", "string", true, false, "JID de origem."),
			field("callerPn", "string | null", true, true, "Phone number do chamador quando disponivel."),
			field("isGroup", "boolean | null", true, true, "Indica chamada em grupo quando o normalizador consegue inferir."),
			field("groupJid", "string | null", true, true, "JID do grupo quando disponivel."),
			field("id", "string", true, false, "ID da chamada."),
			field("date", "string", true, false, "Timestamp RFC3339 da chamada/processamento."),
			field("isVideo", "boolean | null", true, true, "Indica chamada de video quando o normalizador consegue inferir."),
			field("status", "string", true, false, "Status normalizado da chamada.", "offer", "ringing", "preaccept", "transport", "relaylatency", "timeout", "reject", "accept", "terminate", "unknown"),
			field("offline", "boolean", true, false, "Indica se o evento veio como offline."),
			field("latencyMs", "number | null", true, true, "Latencia em milissegundos quando reportada."),
		},
		PossibleValues: []PossibleEnum{{Field: "status", Values: []string{"offer", "ringing", "preaccept", "transport", "relaylatency", "timeout", "reject", "accept", "terminate", "unknown"}}},
		Example: envelope(dbtypes.WebhookEventCallUpsert, map[string]any{
			"chatId":    "5511999999999@s.whatsapp.net",
			"from":      "5511999999999@s.whatsapp.net",
			"callerPn":  "5511999999999",
			"isGroup":   false,
			"groupJid":  nil,
			"id":        "3EB0C4D0A1",
			"date":      "2026-07-04T19:05:00Z",
			"isVideo":   false,
			"status":    "offer",
			"offline":   false,
			"latencyMs": nil,
		}),
		ImplementedIn: []string{
			"internal/whatsapp/service.go",
			"internal/whatsapp/webhook_extended_events.go",
			"internal/webhook/payload.go",
		},
	}
}

func labelsAssociationDoc() EventDoc {
	return EventDoc{
		Name:           string(dbtypes.WebhookEventLabelsAssociation),
		Flag:           "labelsAssociation",
		Description:    "Associacao ou remocao de label em chat ou mensagem.",
		InternalEvents: []string{"*events.LabelAssociationChat", "*events.LabelAssociationMessage"},
		Persistence:    "Nao persiste dados especificos antes da entrega do webhook.",
		DataType:       "object",
		DataSchema:     "LabelsAssociationWebhookData",
		DynamicFields:  true,
		Fields: []Field{
			field("type", "string", true, false, "Tipo de associacao.", "chat", "message"),
			field("chatJid", "string", true, false, "JID da conversa."),
			field("messageId", "string", false, false, "ID da mensagem quando type=message."),
			field("labelId", "string", true, false, "ID da label."),
			field("action", "string", false, false, "Acao inferida quando labeled esta presente.", "add", "remove"),
			field("dateTime", "string", true, false, "Timestamp RFC3339 do processamento."),
			field("additionalProperties", "object", false, false, "Campos achatados do evento original."),
		},
		PossibleValues: []PossibleEnum{
			{Field: "type", Values: []string{"chat", "message"}},
			{Field: "action", Values: []string{"add", "remove"}},
		},
		Example: envelope(dbtypes.WebhookEventLabelsAssociation, map[string]any{
			"type":     "chat",
			"chatJid":  "5511999999999@s.whatsapp.net",
			"labelId":  "7",
			"action":   "add",
			"dateTime": "2026-07-04T18:00:00Z",
		}),
		ImplementedIn: []string{
			"internal/whatsapp/service.go",
			"internal/webhook/payload.go",
		},
	}
}

func labelsEditDoc() EventDoc {
	return EventDoc{
		Name:           string(dbtypes.WebhookEventLabelsEdit),
		Flag:           "labelsEdit",
		Description:    "Criacao, alteracao ou remocao de label.",
		InternalEvents: []string{"*events.LabelEdit"},
		Persistence:    "Nao persiste dados especificos antes da entrega do webhook.",
		DataType:       "object",
		DataSchema:     "LabelsEditWebhookData",
		DynamicFields:  true,
		Fields: []Field{
			field("id", "string", true, false, "ID da label, derivado de labelId."),
			field("name", "string", false, false, "Nome da label quando informado."),
			field("color", "number", false, false, "Cor da label quando informada."),
			field("deleted", "boolean", false, false, "Indica label removida quando informado."),
			field("additionalProperties", "object", false, false, "Campos achatados do evento original."),
		},
		Example: envelope(dbtypes.WebhookEventLabelsEdit, map[string]any{
			"id":      "12",
			"name":    "Cliente",
			"color":   3,
			"deleted": false,
		}),
		Notes: []string{"O normalizador nao adiciona campo `type` nem `dateTime` para este evento."},
		ImplementedIn: []string{
			"internal/whatsapp/service.go",
			"internal/whatsapp/webhook_extended_events.go",
		},
	}
}

func profilePictureUpdateDoc() EventDoc {
	return EventDoc{
		Name:           string(dbtypes.WebhookEventProfilePictureUpdated),
		Flag:           "profilePictureUpdated",
		Description:    "Atualizacao de foto de perfil da propria instancia ou de outro JID.",
		InternalEvents: []string{"*events.Picture"},
		Persistence:    "Quando o JID e a propria instancia, atualiza profilePicUrl da instancia antes da entrega. Para outros JIDs nao ha persistencia especifica.",
		DataType:       "object",
		DataSchema:     "ProfilePictureUpdatedWebhookData",
		Fields: []Field{
			field("jid", "string", true, false, "JID que teve a foto alterada."),
			field("author", "string", false, false, "JID do autor quando informado; omitido quando vazio."),
			field("dateTime", "string", true, false, "Timestamp RFC3339 UTC do evento ou do processamento."),
			field("remove", "boolean", true, false, "Indica remocao de foto."),
			field("pictureId", "string", false, false, "ID da foto quando informado; omitido quando vazio."),
			field("isGroup", "boolean", true, false, "Indica se o JID pertence a grupo."),
		},
		Example: envelope(dbtypes.WebhookEventProfilePictureUpdated, map[string]any{
			"jid":       "120363000000000000@g.us",
			"author":    "5531999999999@s.whatsapp.net",
			"dateTime":  "2026-07-04T18:00:00Z",
			"remove":    false,
			"pictureId": "pic-123",
			"isGroup":   true,
		}),
		ImplementedIn: []string{
			"internal/whatsapp/service.go",
			"internal/webhook/payload.go",
		},
	}
}

func userAboutUpdateDoc() EventDoc {
	return EventDoc{
		Name:           string(dbtypes.WebhookEventUserAboutUpdated),
		Flag:           "userAboutUpdated",
		Description:    "Atualizacao do recado/about de um usuario.",
		InternalEvents: []string{"*events.UserAbout"},
		Persistence:    "Nao persiste dados especificos antes da entrega do webhook.",
		DataType:       "object",
		DataSchema:     "UserAboutUpdatedWebhookData",
		Fields: []Field{
			field("jid", "string", true, false, "JID do usuario."),
			field("status", "string", false, false, "Texto do about quando informado."),
			field("dateTime", "string", true, false, "Timestamp RFC3339 do processamento."),
		},
		Example: envelope(dbtypes.WebhookEventUserAboutUpdated, map[string]any{
			"jid":      "5511999999999@s.whatsapp.net",
			"status":   "Disponivel",
			"dateTime": "2026-07-04T18:00:00Z",
		}),
		ImplementedIn: []string{
			"internal/whatsapp/service.go",
			"internal/webhook/payload.go",
		},
	}
}

func identityUpdateDoc() EventDoc {
	return EventDoc{
		Name:           string(dbtypes.WebhookEventIdentityUpdated),
		Flag:           "identityUpdated",
		Description:    "Mudanca de identidade criptografica de um contato.",
		InternalEvents: []string{"*events.IdentityChange"},
		Persistence:    "Nao persiste dados especificos antes da entrega do webhook.",
		DataType:       "object",
		DataSchema:     "IdentityUpdatedWebhookData",
		Fields: []Field{
			field("jid", "string", true, false, "JID cuja identidade mudou."),
			field("dateTime", "string", true, false, "Timestamp RFC3339 UTC do evento ou do processamento."),
			field("implicit", "boolean", true, false, "Indica mudanca implicita reportada pelo whatsmeow."),
		},
		Example: envelope(dbtypes.WebhookEventIdentityUpdated, map[string]any{
			"jid":      "5511999999999@s.whatsapp.net",
			"dateTime": "2026-07-04T18:00:00Z",
			"implicit": true,
		}),
		ImplementedIn: []string{
			"internal/whatsapp/service.go",
			"internal/webhook/payload.go",
		},
	}
}

func mediaRetryDoc() EventDoc {
	return EventDoc{
		Name:           string(dbtypes.WebhookEventMediaRetry),
		Flag:           "mediaRetry",
		Description:    "Resultado ou erro relacionado a tentativa de retry de midia.",
		InternalEvents: []string{"*events.MediaRetry"},
		Persistence:    "Nao persiste dados especificos antes da entrega do webhook.",
		DataType:       "object",
		DataSchema:     "MediaRetryWebhookData",
		Fields: []Field{
			field("messageId", "string", true, false, "ID da mensagem."),
			field("chatJid", "string", true, false, "JID da conversa."),
			field("senderJid", "string", false, false, "JID do remetente quando disponivel; omitido quando ausente."),
			field("keyFromMe", "boolean", true, false, "Indica se a mensagem e da propria instancia."),
			field("hasCiphertext", "boolean", true, false, "Indica se o evento carregou ciphertext."),
			field("errorCode", "number", false, false, "Codigo de erro quando informado; omitido quando ausente."),
			field("dateTime", "string", true, false, "Timestamp RFC3339 UTC do evento ou do processamento."),
		},
		Example: envelope(dbtypes.WebhookEventMediaRetry, map[string]any{
			"messageId":     "ABC123",
			"chatJid":       "5511999999999@s.whatsapp.net",
			"senderJid":     "5511988888888@s.whatsapp.net",
			"keyFromMe":     false,
			"hasCiphertext": true,
			"errorCode":     404,
			"dateTime":      "2026-07-04T18:00:00Z",
		}),
		Notes: []string{"Ciphertext e IV recebidos pelo whatsmeow nao sao expostos no webhook."},
		ImplementedIn: []string{
			"internal/whatsapp/service.go",
			"internal/whatsapp/webhook_events.go",
			"internal/webhook/payload.go",
		},
	}
}

func settingsUpdateDoc() EventDoc {
	return EventDoc{
		Name:           string(dbtypes.WebhookEventSettingsUpdated),
		Flag:           "settingsUpdated",
		Description:    "Atualizacao de configuracoes do usuario/instancia.",
		InternalEvents: []string{"*events.PushNameSetting", "*events.UserStatusMute"},
		Persistence:    "Nao persiste dados especificos antes da entrega do webhook.",
		DataType:       "object",
		DataSchema:     "SettingsUpdatedWebhookData",
		Fields: []Field{
			field("type", "string", true, false, "Subtipo de configuracao.", "push.name", "user.status.mute"),
			field("jid", "string", false, false, "JID afetado quando o subtipo informar."),
			field("name", "string", false, false, "Nome configurado no subtipo push.name."),
			field("muted", "boolean", false, false, "Estado de mute no subtipo user.status.mute."),
			field("fromFullSync", "boolean", true, false, "Indica se veio de sincronizacao completa."),
			field("dateTime", "string", true, false, "Timestamp RFC3339 UTC do evento ou do processamento."),
		},
		PossibleValues: []PossibleEnum{{Field: "type", Values: []string{"push.name", "user.status.mute"}}},
		Example: envelope(dbtypes.WebhookEventSettingsUpdated, map[string]any{
			"type":         "push.name",
			"name":         "Minha instancia",
			"fromFullSync": false,
			"dateTime":     "2026-07-04T18:00:00Z",
		}),
		ImplementedIn: []string{
			"internal/whatsapp/service.go",
			"internal/whatsapp/webhook_events.go",
			"internal/webhook/payload.go",
		},
	}
}

func messageFields() []Field {
	return []Field{
		field("id", "number", true, false, "ID interno da mensagem."),
		field("keyRemoteJid", "string | null", true, true, "JID remoto da mensagem."),
		field("keyLid", "string | null", true, true, "LID remoto da mensagem."),
		field("keyFromMe", "boolean", true, false, "Indica se a mensagem foi enviada pela propria instancia."),
		field("keyParticipant", "string | null", true, true, "Participante em mensagens de grupo."),
		field("keyParticipantLid", "string | null", true, true, "LID do participante em mensagens de grupo."),
		field("pushName", "string | null", true, true, "Nome exibido do remetente quando conhecido."),
		field("messageType", "string", true, false, "Tipo normalizado da mensagem."),
		field("content", "object", true, false, "Conteudo normalizado da mensagem."),
		field("messageTimestamp", "number", true, false, "Timestamp Unix em segundos."),
		field("device", "string | null", true, true, "Dispositivo/origem inferida da mensagem."),
		field("isGroup", "boolean", true, false, "Indica se a mensagem pertence a grupo."),
		field("metadata", "object | null", true, true, "Metadados adicionais normalizados."),
	}
}

func contactUpsertFields() []Field {
	return []Field{
		field("id", "number", true, false, "ID interno do contato persistido."),
		field("remoteJid", "string", true, false, "JID remoto do contato."),
		field("lid", "string | null", true, true, "LID do contato quando conhecido."),
		field("pushName", "string | null", true, true, "Push name salvo para o contato."),
		field("profilePicUrl", "string | null", true, true, "URL de foto do perfil quando conhecida."),
		field("action", "string", true, false, "Acao executada.", "upserted"),
	}
}

func contactUpdateFields() []Field {
	return []Field{
		field("id", "number", true, false, "ID interno do contato persistido."),
		field("remoteJid", "string", true, false, "JID remoto do contato."),
		field("lid", "string | null", true, true, "LID do contato quando conhecido."),
		field("pushName", "string | null", false, true, "Push name atualizado quando presente."),
		field("businessName", "string | null", false, true, "Nome comercial atualizado quando presente."),
		field("action", "string", true, false, "Acao executada.", "updated"),
		field("source", "string", true, false, "Origem da alteracao.", "pushName", "businessName"),
	}
}

func groupParticipantFields() []Field {
	return []Field{
		field("id", "string", false, false, "JID tradicional do participante quando disponivel; omitido quando ausente."),
		field("lid", "string", false, false, "LID do participante quando conhecido; omitido quando ausente."),
		field("isAdmin", "boolean", true, false, "Indica se e admin."),
		field("isSuperAdmin", "boolean", true, false, "Indica se e super admin."),
		field("admin", "string | null", true, true, "Valor bruto do nivel de admin quando informado."),
	}
}

func groupUpsertFields() []Field {
	return append([]Field{
		field("id", "string", true, false, "JID do grupo."),
		field("subject", "string", true, false, "Nome do grupo."),
		field("participants", "GroupParticipantWebhookData[]", true, false, "Participantes conhecidos do grupo."),
	}, groupPartialFieldSet()...)
}

func groupUpdateFields() []Field {
	return append([]Field{
		field("partial", "GroupPartialWebhookData", true, false, "Metadados parciais alterados no grupo."),
	}, prefixFields("partial.", groupPartialFieldSet())...)
}

func groupPartialFieldSet() []Field {
	return []Field{
		field("notify", "string", false, false, "Nome de notificacao do grupo quando informado."),
		field("addressingMode", "string", false, false, "Modo de enderecamento do grupo quando informado."),
		field("owner", "string", false, false, "JID do owner quando informado."),
		field("ownerPn", "string", false, false, "Phone number do owner quando informado."),
		field("ownerUsername", "string", false, false, "Username do owner quando informado."),
		field("ownerCountryCode", "string", false, false, "Codigo de pais do owner quando informado."),
		field("subjectOwner", "string", false, false, "JID de quem definiu o subject quando informado."),
		field("subjectOwnerPn", "string", false, false, "Phone number de quem definiu o subject quando informado."),
		field("subjectOwnerUsername", "string", false, false, "Username de quem definiu o subject quando informado."),
		field("subjectTime", "number", false, false, "Timestamp Unix do subject quando informado."),
		field("creation", "number", false, false, "Timestamp Unix de criacao quando informado."),
		field("desc", "string", false, false, "Descricao do grupo quando informada."),
		field("descOwner", "string", false, false, "JID de quem definiu a descricao quando informado."),
		field("descOwnerPn", "string", false, false, "Phone number de quem definiu a descricao quando informado."),
		field("descOwnerUsername", "string", false, false, "Username de quem definiu a descricao quando informado."),
		field("descId", "string", false, false, "ID da descricao quando informado."),
		field("descTime", "number", false, false, "Timestamp Unix da descricao quando informado."),
		field("linkedParent", "string", false, false, "Grupo/comunidade pai quando informado."),
		field("restrict", "boolean", false, false, "Restricao de edicao quando informada."),
		field("announce", "boolean", false, false, "Modo anuncio quando informado."),
		field("memberAddMode", "boolean", false, false, "Modo de adicao por membros quando informado."),
		field("joinApprovalMode", "boolean", false, false, "Modo de aprovacao de entrada quando informado."),
		field("isCommunity", "boolean", false, false, "Indica comunidade quando informado."),
		field("isCommunityAnnounce", "boolean", false, false, "Indica grupo de anuncios da comunidade quando informado."),
		field("size", "number", false, false, "Tamanho do grupo quando informado."),
		field("ephemeralDuration", "number", false, false, "Duracao de mensagens temporarias em segundos quando informada."),
		field("inviteCode", "string", false, false, "Codigo de convite quando informado."),
		field("author", "string", false, false, "Autor da alteracao quando informado."),
		field("authorPn", "string", false, false, "Phone number do autor quando informado."),
		field("authorUsername", "string", false, false, "Username do autor quando informado."),
	}
}

func prefixFields(prefix string, fields []Field) []Field {
	prefixed := make([]Field, len(fields))
	for i, item := range fields {
		item.Name = prefix + item.Name
		prefixed[i] = item
	}
	return prefixed
}

func supportedEventNames() []string {
	events := dbtypes.SupportedWebhookEvents()
	names := make([]string, 0, len(events))
	for _, event := range events {
		names = append(names, string(event))
	}
	sort.Strings(names)
	return names
}

func field(name, typ string, required, nullable bool, description string, values ...string) Field {
	return Field{
		Name:        name,
		Type:        typ,
		Required:    required,
		Nullable:    nullable,
		Description: description,
		Values:      values,
	}
}

func envelope(event dbtypes.WebhookEvent, data any) map[string]any {
	return envelopeWithInstance(event, map[string]any{
		"id":                 1,
		"name":               "codechat",
		"connectionStatus":   "online",
		"ownerJid":           "5511999999999@s.whatsapp.net",
		"externalAttributes": map[string]any{},
	}, data)
}

func envelopeWithInstance(event dbtypes.WebhookEvent, instance map[string]any, data any) map[string]any {
	return map[string]any{
		"event":     string(event),
		"instance":  instance,
		"data":      data,
		"timestamp": "2026-07-04T18:00:00Z",
	}
}
