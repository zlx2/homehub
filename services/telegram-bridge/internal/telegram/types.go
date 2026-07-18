package telegram

type Update struct {
	UpdateID    int64    `json:"update_id"`
	Message     *Message `json:"message,omitempty"`
	ChannelPost *Message `json:"channel_post,omitempty"`
}

func (u Update) Content() *Message {
	if u.Message != nil {
		return u.Message
	}
	return u.ChannelPost
}

type Message struct {
	MessageID    int64       `json:"message_id"`
	From         *User       `json:"from,omitempty"`
	SenderChat   *Chat       `json:"sender_chat,omitempty"`
	Chat         Chat        `json:"chat"`
	Date         int64       `json:"date"`
	Text         string      `json:"text,omitempty"`
	Caption      string      `json:"caption,omitempty"`
	MediaGroupID string      `json:"media_group_id,omitempty"`
	Photo        []PhotoSize `json:"photo,omitempty"`
	Animation    *FileMedia  `json:"animation,omitempty"`
	Audio        *FileMedia  `json:"audio,omitempty"`
	Document     *FileMedia  `json:"document,omitempty"`
	Video        *FileMedia  `json:"video,omitempty"`
	Voice        *FileMedia  `json:"voice,omitempty"`
	VideoNote    *FileMedia  `json:"video_note,omitempty"`
	Sticker      *Sticker    `json:"sticker,omitempty"`
}

type User struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name,omitempty"`
	Username  string `json:"username,omitempty"`
}

type Chat struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title,omitempty"`
	Username  string `json:"username,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
}

type PhotoSize struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	FileSize     int64  `json:"file_size,omitempty"`
}

type FileMedia struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	FileName     string `json:"file_name,omitempty"`
	MIMEType     string `json:"mime_type,omitempty"`
	FileSize     int64  `json:"file_size,omitempty"`
}

type Sticker struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	Type         string `json:"type,omitempty"`
	IsAnimated   bool   `json:"is_animated"`
	IsVideo      bool   `json:"is_video"`
	FileSize     int64  `json:"file_size,omitempty"`
}

type File struct {
	FileID   string `json:"file_id"`
	FileSize int64  `json:"file_size,omitempty"`
	FilePath string `json:"file_path,omitempty"`
}

type Bot struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	Username  string `json:"username,omitempty"`
}
