package backend


// Peer represents a discovered node in the local subnet.
type Peer struct {
	ID       string    `json:"id"`
	Hostname string    `json:"hostname"`
	IP       string    `json:"ip"`
	Port     int       `json:"port"`
	IsSelf   bool   `json:"isSelf"`
	LastSeen int64  `json:"lastSeen"`
}

// DropType defines the category of dropped content.
type DropType string

const (
	DropTypeSnippet DropType = "snippet"
	DropTypeFile    DropType = "file"
)

// DropSnippetPayload defines metadata and payload for code snippets and terminal commands.
type DropSnippetPayload struct {
	ID        string `json:"id"`
	Sender    string `json:"sender"`
	SenderIP  string `json:"senderIp"`
	Syntax    string `json:"syntax"`
	Content   string `json:"content"`
	Timestamp int64  `json:"timestamp"`
}

// DropFilePayload defines metadata for transferred files and folders.
type DropFilePayload struct {
	ID        string `json:"id"`
	Sender    string `json:"sender"`
	SenderIP  string `json:"senderIp"`
	FileName  string `json:"fileName"`
	FileSize  int64  `json:"fileSize"`
	FilePath  string `json:"filePath"`
	IsArchive bool   `json:"isArchive"`
	Timestamp int64  `json:"timestamp"`
}

// DropItem is the unified model published to the frontend feed.
type DropItem struct {
	ID        string              `json:"id"`
	Type      DropType            `json:"type"`
	Sender    string              `json:"sender"`
	SenderIP  string              `json:"senderIp"`
	Timestamp int64               `json:"timestamp"`
	Snippet   *DropSnippetPayload `json:"snippet,omitempty"`
	File      *DropFilePayload    `json:"file,omitempty"`
}
