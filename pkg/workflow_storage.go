package diplomat

type JobOutputStore struct {
	data map[string]*Output{}
}

func NewJobOutputStore() *JobOutputStore {
	return &JobOutputStore{map[string]*Output{}}
}

func (ws *JobOutputStore) RestoreJobResult(id string) (*Output, error) {
	return ws.data[id], nil
}

func (ws *JobOutputStore) SaveJobResult(id string, out *Output) error {
	ws.data[id] = out
	return nil
}

type ChannelEventLog interface {
	Write(id string, evt Event)
	Read(f func(evt Event) error) error
}

type inmemoryChannelEventLog struct {
	// EventLog should be separated and unique per Diplomat channel
	// Otherwise events from mixed channels are replied, which is obviously a bug!
	channelId string

	pos int
	data []UnprocessedEvent
}

func (l *inmemoryChannelEventLog) Write(id string, evt Event) {
	l.data = append(l.data, UnprocessedEvent{id: id, evt: evt})
}

func (l *inmemoryChannelEventLog) Read(f func(evt Event) error) error {
	d := l.data[l.pos]
	if err := f(d.evt); err != nil {
		return err
	}
	l.pos ++
	return nil
}

type UnprocessedEvent struct {
	id string
	evt Event
}
