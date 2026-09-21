# gemini - довідник

Повний довідник пакета `gemini`: клієнт, спільна модель `goloop/ai`, генерація
контенту (інтерфейс і нативна), стрімінг, embeddings, підрахунок токенів і
моделі.

Англійська версія: **[DOC.md](DOC.md)**.

## Зміст

- [Ментальна модель](#ментальна-модель)
- [Створення клієнта](#створення-клієнта)
- [Generate і Stream](#generate-і-stream)
- [Структурований вивід](#структурований-вивід)
- [Пошук на боці провайдера](#пошук-на-боці-провайдера)
- [Здатності й відмови на рівні моделі](#здатності-й-відмови-на-рівні-моделі)
- [Нативний generateContent](#нативний-generatecontent)
- [Інструменти та результати](#інструменти-та-результати)
- [Зображення й system-промпти](#зображення-й-system-промпти)
- [Генерація зображень](#генерація-зображень)
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

Якщо стрім обірвано до того, як кандидат отримає `finishReason`, `Stream` віддає
`io.ErrUnexpectedEOF`, а не повідомляє обрізану чи порожню відповідь як
завершену. Подія, що не є коректним JSON, так само завершує стрім помилкою;
`StreamGenerateContent` повідомляє про неї так само.

## Структурований вивід

`ai.Request.Format` лягає на власний `responseMimeType` провайдера, тож запит на JSON
провайдер **дотримує**, а не просто «чує»:

```go
resp, err := c.Generate(ctx, &ai.Request{
	Model:    "the-model",
	Messages: []ai.Message{ai.UserText("Склади SEO-поля для цієї статті.")},
	Format: &ai.Format{
		Type:   ai.FormatJSONSchema,
		Name:   "seo",
		Schema: schema,
	},
})

var seo SEO
err = resp.JSON(&seo)
```

`ai.FormatJSON` виставляє `responseMimeType` у `application/json`;
`ai.FormatJSONSchema` кладе схему поруч, у `responseJsonSchema`.

API має **два** поля для схеми, і вони взаємовиключні. `responseSchema` приймає
підмножину **OpenAPI 3.0**, яка збігається з JSON Schema лише для найпростіших
форм - `$ref`, `oneOf`, `$defs` і `additionalProperties` це те, де вони
розходяться. `ai.Format.Schema` - це JSON Schema, тож драйвер бере
`responseJsonSchema`, поле, що говорить цим діалектом.
`GenerationConfig.ResponseSchema` лишається для тих, хто будує `GenerateRequest`
власноруч і має на увазі саме OpenAPI.


`ai.Response.Format` дорівнює `ai.FormatNative`: цей провайдер дотримує кожну
форму, яку приймає. Які моделі підтримують схемний режим - справа провайдера;
таблиці можливостей тут немає, тож про непідтримувану пару скаже він сам.

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

## Генерація зображень

Imagen генерує зображення через ендпоінт predict. Читайте кожне зображення
через `Bytes`, так само як з іншими goloop image-драйверами:

```go
resp, err := c.GenerateImage(ctx, &gemini.ImageRequest{
	Model:  gemini.ModelImagen4,
	Prompt: "a watercolor cat",
	N:      1,
	Size:   "1536x1024", // мапиться в найближче співвідношення
})
png, err := resp.Data[0].Bytes()
```

Imagen працює зі співвідношеннями сторін, а не точними пікселями, тож `Size`
мапиться в найближче з `1:1`, `3:4`, `4:3`, `9:16`, `16:9` - Google не гарантує
точних пікселів, а відхиляти розумний `1024x1024` було б неввічливо. Задайте
`AspectRatio`, щоб обрати котресь напряму; коли задано обидва, виграє
`AspectRatio`. Nil-запит або запит без промпту відхиляється до мережі
(`ErrNoImageRequest`, `ErrNoImagePrompt`).

`ImageResponse.Usage` присутній для однорідності з іншими драйверами, але тут
завжди nil: ендпоінт predict не звітує токени. `Bytes`, `ErrNoImageBytes` і
`ImageUsage` мають ті самі імена, що й в інших image-драйверах, тож зміна
провайдера не міняє того, як читається результат.

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

## Пошук на боці провайдера

`ai.Request.Hosted` лягає на заземлення через Google Search, яке їде окремим
записом списку інструментів поруч із вашими оголошеннями функцій:

```go
resp, err := c.Generate(ctx, &ai.Request{
	Model:    gemini.ModelGemini25Flash,
	Messages: []ai.Message{ai.UserText("Що вийшло цього тижня?")},
	Hosted:   []ai.Hosted{{Kind: ai.HostedWebSearch}},
})
for _, s := range resp.Citations() {
	fmt.Println(s.URL, resp.Text()[s.StartByte:s.EndByte])
}
```

Пошук виконується на боці провайдера, тож до циклу інструментів не потрапляє
нічого нового.

Джерела приходять як `ai.Citation` на тексті, який вони підтверджують, і цей
провайдер незвичний тим, що каже *де саме*: його зсуви - байтові, тож
`StartByte` і `EndByte` ріжуть `ai.Text.Text` точно, і можна показати, яке
речення підпирає кожне джерело. Слів самого джерела він не дає ніколи, тому
`CitedText` тут лишається порожнім. Діапазон, який не потрапляє у свою частину,
відкидається, а не повідомляється хибним.

Виняток - стрім: він несе джерела без діапазону, бо провайдер міряє діапазон
відносно завершеної відповіді, а в стріму її ще немає.

`ai.Response.Hosted` каже, чи заземлення відбулося і скільки запитів воно
виконало, - саме це провайдер тарифікує.

Заземлення тут не має жодних налаштувань (провайдер сам вирішує, скільки
пошуків робити й по чому), тож запит, який задає `ai.HostedWeb.MaxUses`,
будь-який зі списків доменів чи регіон, стає `ai.ErrNoHosted`, а не пошуком,
який тихо це ігнорує.

Пошук і структурований `ai.Format` не можна просити в одному виклику: цей
провайдер забезпечує формат, обмежуючи те, що модель може видати, а це не
тримається разом із заземленням. Така пара стає `ai.ErrFormatWithHosted` ще до
відправлення. Два виклики працюють: перший із пошуком і вільним текстом, другий
без `Hosted`, який перекладає його в схему.

## Здатності й відмови на рівні моделі

Драйвер реалізує `ai.Capable`. `ai.CapabilitiesOf(c)` звітує, що він виконує і
які налаштування приймає, а `ai.SupportsHosted` відповідає до виклику, чи
запит у такому вигляді відомо-працездатний, - це рішення про те, чи показувати
контрол пошуку взагалі, і про один запит чи два.

Звітується тут: пошук без жодних налаштувань - провайдер сам вирішує, скільки
шукати й по чому, - і свідомо порожній `WithFormat`: обмеження, що забезпечує
схему, не тримається разом із заземленням, тож фіча, якій треба обидва, робить
два виклики (`ai.ErrFormatWithHosted` каже це в рантаймі).

Це підказка, не дозвіл: підтримка залежить і від моделі, і від акаунта, і від
регіону, тож `ai.ErrNoHosted` та `ai.ErrNoFormat` лишаються джерелом істини.
Поруч змінилося ось що: відмова, яку провайдер звітує лише 400-кою («ця модель
такого не вміє»), тепер приходить загорнутою в ті самі сентинели, тож один
`errors.Is` покриває і обмеження, відоме драйверу наперед, і вивчене з дроту.
Власний `ai.APIError` провайдера лишається досяжним через `errors.As`.
Загортання свідомо вузьке: лише 400, лише здатність, про яку запит справді
просив, лише помилка, що називає рівно цю фічу.

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
