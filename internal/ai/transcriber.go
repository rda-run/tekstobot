package ai

// Transcriber abstracts audio transcription backends.
type Transcriber interface {
	ProcessAudio(filePath string) (string, error)
}
