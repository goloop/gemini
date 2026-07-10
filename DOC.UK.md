# gemini - довідник

Повний довідник пакета `gemini`: клієнт, спільна модель `goloop/ai`, генерація
контенту (інтерфейс і нативна), стрімінг, embeddings, підрахунок токенів і
моделі.

Англійська версія: **[DOC.md](DOC.md)**.

## Зміст

- [Ментальна модель](#ментальна-модель)
- [Створення клієнта](#створення-клієнта)
- [Generate і Stream](#generate-і-stream)
- [Нативний generateContent](#нативний-generatecontent)
- [Інструменти та результати](#інструменти-та-результати)
- [Зображення й system-промпти](#зображення-й-system-промпти)
- [Embeddings](#embeddings)
- [Підрахунок токенів](#підрахунок-токенів)
- [Моделі](#моделі)
- [Опції та помилки](#опції-та-помилки)

## Ментальна модель

`gemini.Client` реалізує `ai.Client` - провайдер-незалежний контракт із
`github.com/goloop/ai`. Спільні `Generate` і `Stream` покривають спільну основу
(чат із інструментами, зображеннями й стрімінгом), тож код проти інтерфейсу
працює з будь-яким провайдером.

Специфіка Gemini - у нативних методах: повний `GenerateContent` із generation
config і схемою відповіді, embeddings, підрахунок токенів, перелік моделей. Їх
немає у спільному інтерфейсі.

```go
import (
	"github.com/goloop/ai"
	"github.com/goloop/gemini"
)
```

## Створення клієнта

```go
c := gemini.New(os.Getenv("GEMINI_API_KEY"))

c = gemini.New(apiKey, gemini.WithTimeout(30*time.Second))
```

Base URL за замовчуванням `https://generativelanguage.googleapis.com/v1beta`.
Ключ передається в заголовку `x-goog-api-key`. Наведіть `WithBaseURL` на
будь-який сумісний ендпоінт, щоб перевикористати клієнт.

## Generate і Stream

```go
resp, err := c.Generate(ctx, &ai.Request{
	Model:    gemini.ModelGemini25Flash,
	System:   "You are concise.",
	Messages: []ai.Message{ai.UserText("Name three primary colors.")},
})
resp.Text()
resp.ToolCalls()
resp.Usage
```

`Stream` повертає `iter.Seq2[ai.Chunk, error]`: текстові дельти чанками з `Text`,
виклик інструмента - чанком із `ToolCall`, фінальний чанк - `Done` і `Usage`.

```go
for chunk, err := range c.Stream(ctx, req) {
	if err != nil {
		return err
	}
	fmt.Print(chunk.Text)
}
```

## Нативний generateContent

Для опцій, специфічних для Gemini, будуйте `GenerateRequest` і викликайте
`GenerateContent` чи `StreamGenerateContent`:

```go
resp, err := c.GenerateContent(ctx, gemini.ModelGemini25Flash, &gemini.GenerateRequest{
	Contents: []gemini.Content{
		{Role: "user", Parts: []gemini.Part{{Text: "List two colors."}}},
	},
	GenerationConfig: &gemini.GenerationConfig{
		MaxOutputTokens:  256,
		ResponseMIMEType: "application/json",
	},
})
resp.Text()
```

Ролі - `"user"` і `"model"`; system-промпт живе в `SystemInstruction`. `Part`
містить рівно одне з `Text`, `InlineData`, `FileData`, `FunctionCall` чи
`FunctionResponse`.

## Інструменти та результати

Інструменти використовують спільний тип `ai.Tool`; `ToolChoice` мапиться на
режим виклику функцій Gemini (`ToolAuto` -> `AUTO`, `ToolNone` -> `NONE`,
`ToolRequired` -> `ANY`).

Gemini зіставляє результат із викликом за іменем функції, а не за ID. Драйвер це
приховує: повернений `ai.ToolUse` несе ім'я функції як свій `ID`, тож відповідь
через `ai.ToolResult` із тим самим `ID` маршрутизується правильно.

```go
for _, call := range resp.ToolCalls() {
	req.Messages = append(req.Messages, ai.Message{
		Role:  ai.RoleTool,
		Parts: []ai.Part{ai.ToolResult{ID: call.ID, Content: `{"tempC":21}`}},
	})
}
```

Результат, чий `Content` є JSON-об'єктом, передається як є; будь-який інший
рядок загортається в `{"result": "..."}` (або `{"error": "..."}`, якщо
`IsError`).

## Зображення й system-промпти

Вбудовані байти зображення стають частиною `inlineData`; `ai.Image` з `URL` -
посиланням `fileData`. System-текст із поля `System` чи повідомлення
`RoleSystem` зливається в системну інструкцію запиту.

```go
ai.Message{Role: ai.RoleUser, Parts: []ai.Part{
	ai.Text{Text: "What is in this image?"},
	ai.Image{MIME: "image/png", Data: pngBytes},
}}
```

## Embeddings

```go
vecs, err := c.Embed(ctx, "text-embedding-004", "hello", "world")
e, err := c.EmbedContent(ctx, "text-embedding-004", &gemini.EmbedRequest{
	Content:              gemini.Content{Parts: []gemini.Part{{Text: "hello"}}},
	TaskType:             "RETRIEVAL_DOCUMENT",
	OutputDimensionality: 256,
})
e.Values
```

## Підрахунок токенів

```go
n, err := c.CountTokens(ctx, gemini.ModelGemini25Flash, &gemini.GenerateRequest{
	Contents: []gemini.Content{
		{Role: "user", Parts: []gemini.Part{{Text: "hello there"}}},
	},
})
```

## Моделі

```go
models, err := c.Models(ctx)
m, err := c.GetModel(ctx, gemini.ModelGemini25Flash)
m.InputTokenLimit
```

## Опції та помилки

Опції: `WithBaseURL`, `WithHTTPClient`, `WithTimeout`, `WithMaxRetries`,
`WithHeader`.

Невдала відповідь стає `*ai.APIError` зі `Status`, `Type` (рядок статусу Gemini,
напр. `INVALID_ARGUMENT`), `Message` і сирим тілом:

```go
var apiErr *ai.APIError
if errors.As(err, &apiErr) && apiErr.Status == http.StatusTooManyRequests {
	// backoff
}
```

Запити без моделі чи повідомлень падають до мережі з `ai.ErrNoModel` або
`ai.ErrNoMessages`.

Коли Gemini блокує промпт із міркувань безпеки, він повертає HTTP 200 без
кандидатів і з причиною блокування. Драйвер перетворює це на `*ai.APIError`
(статус 400), чиє повідомлення несе причину блокування, тож заблокований промпт
- це звичайна помилка, а не тиха порожня відповідь.
