package tui

type ChannelLogWriter struct {
	LogChannel chan<- string
}

func (w ChannelLogWriter) Write(p []byte) (n int, err error) {
	w.LogChannel <- string(p)
	return len(p), nil
}
