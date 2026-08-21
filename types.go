package main

type ArticleMetadata struct {
	ArticleUUID string  `json:"article_uuid"`
	Title       string  `json:"title"`
	Abstract    string  `json:"abstract"`
	CreatedAt   float64 `json:"created_at"`
	ModifiedAt  float64 `json:"modified_at"`
}

type TemplateMetadata struct {
	TemplateUUID   string  `json:"template_uuid"`
	Name           string  `json:"name"`
	Intro          string  `json:"intro"`
	TitlePrompt    string  `json:"title_prompt"`
	AbstractPrompt string  `json:"abstract_prompt"`
	ContentPrompt  string  `json:"content_prompt"`
	CreatedAt      float64 `json:"created_at"`
	ModifiedAt     float64 `json:"modified_at"`
}

type TemplateSummary struct {
	Name       string  `json:"name"`
	Intro      string  `json:"intro"`
	CreatedAt  float64 `json:"created_at"`
	ModifiedAt float64 `json:"modified_at"`
}

type TemplateSummaryOutput struct {
	Name               string  `json:"name"`
	Intro              string  `json:"intro"`
	CreatedAt          float64 `json:"created_at"`
	ModifiedAt         float64 `json:"modified_at"`
	CreatedAtReadable  string  `json:"created_at_readable,omitempty"`
	ModifiedAtReadable string  `json:"modified_at_readable,omitempty"`
}

type ImageExistsResponse struct {
	Exists bool `json:"exists"`
}

type articlePaths struct {
	ProjectDir          string
	ImagesDir           string
	MetadataPath        string
	ExampleMetadataPath string
	ExampleHTMLPath     string
	ArticleHTMLPath     string
}
