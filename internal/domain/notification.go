package domain

// NotAvailable は、通知メッセージ内で値が存在しないことを示す表示用の文字列です。
const NotAvailable = "N/A"

// NotificationRequest は Slack 等の通知コンポーネントで共有されるデータ構造です。
// 生成された漫画のメタデータを通知先に伝えるために使用します。
type NotificationRequest struct {
	// Command は、実行されたタスク種別です。
	Command string `json:"command,omitempty"`

	// Title は、生成対象の楽曲タイトルです。
	Title string `json:"title,omitempty"`

	// SourceURL は、元になった記事やスクリプトのURLです。
	SourceURL string `json:"source_url"`

	// Mode は、生成に使用したプロンプトモードです。
	Mode string `json:"mode,omitempty"`

	// Seed は、生成に使用された（または生成によって決定された）乱数シード値です。
	Seed *int64 `json:"seed,omitempty"`
}
