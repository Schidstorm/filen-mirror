package filenextra

import (
	"encoding/json"
	"errors"
)

type TypedEvent struct {
	Name string
	Data any
}

func InterpretEvent(eventName string, data map[string]any) (TypedEvent, error) {
	var event any

	switch eventName {
	case "file-new":
		event = &EventSocketFileNew{}
	case "file-rename":
		event = &EventSocketFileRename{}
	case "file-archive-restored":
		event = &EventSocketFileArchiveRestored{}
	case "file-restore":
		event = &EventSocketFileRestore{}
	case "file-move":
		event = &EventSocketFileMove{}
	case "file-trash":
		event = &EventSocketFileTrash{}
	case "file-archived":
		event = &EventSocketFileArchived{}
	case "folder-rename":
		event = &EventSocketFolderRename{}
	case "folder-trash":
		event = &EventSocketFolderTrash{}
	case "folder-move":
		event = &EventSocketFolderMove{}
	case "folder-sub-created":
		event = &EventSocketFolderSubCreated{}
	case "folder-restore":
		event = &EventSocketFolderRestore{}
	case "folder-color-changed":
		event = &EventSocketFolderColorChanged{}
	case "file-deleted-permanent":
		event = &EventSocketFileDeletedPermanent{}
	default:
		return TypedEvent{}, errors.New("unknown event type: " + eventName)
	}

	err := eventFromMap(event, data)
	if err != nil {
		return TypedEvent{}, err
	}

	return TypedEvent{
		Name: eventName,
		Data: event,
	}, nil
}

func eventFromMap(dest any, src map[string]any) error {
	bytes, err := json.Marshal(src)
	if err != nil {
		return err
	}

	return json.Unmarshal(bytes, dest)
}

type EventSocketNew struct {
	UUID      string `json:"uuid"`
	Type      string `json:"type"`
	Timestamp int64  `json:"timestamp"`
	Info      struct {
		IP        string `json:"ip"`
		Metadata  string `json:"metadata"`
		UserAgent string `json:"userAgent"`
		UUID      string `json:"uuid"`
	} `json:"info"`
}

type EventSocketFileNew struct {
	Parent    string         `json:"parent"`
	UUID      string         `json:"uuid"`
	Meta      map[string]any `json:"metadata"`
	RM        string         `json:"rm"`
	Time      int64          `json:"timestamp"`
	Chunks    int            `json:"chunks"`
	Bucket    string         `json:"bucket"`
	Region    string         `json:"region"`
	Version   int            `json:"version"`
	Favorited int            `json:"favorited"`
}

type EventSocketFileRename struct {
	UUID string         `json:"uuid"`
	Meta map[string]any `json:"metadata"`
}

type EventSocketFileArchiveRestored struct {
	CurrentUUID string         `json:"currentUUID"`
	Parent      string         `json:"parent"`
	UUID        string         `json:"uuid"`
	Meta        map[string]any `json:"metadata"`
	RM          string         `json:"rm"`
	Time        int64          `json:"timestamp"`
	Chunks      int            `json:"chunks"`
	Bucket      string         `json:"bucket"`
	Region      string         `json:"region"`
	Version     int            `json:"version"`
	Favorited   int            `json:"favorited"`
}

type EventSocketFileRestore struct {
	Parent    string         `json:"parent"`
	UUID      string         `json:"uuid"`
	Meta      map[string]any `json:"metadata"`
	RM        string         `json:"rm"`
	Time      int64          `json:"timestamp"`
	Chunks    int            `json:"chunks"`
	Bucket    string         `json:"bucket"`
	Region    string         `json:"region"`
	Version   int            `json:"version"`
	Favorited int            `json:"favorited"`
}

type EventSocketFileMove struct {
	Parent    string         `json:"parent"`
	UUID      string         `json:"uuid"`
	Meta      map[string]any `json:"metadata"`
	RM        string         `json:"rm"`
	Time      int64          `json:"timestamp"`
	Chunks    int            `json:"chunks"`
	Bucket    string         `json:"bucket"`
	Region    string         `json:"region"`
	Version   int            `json:"version"`
	Favorited int            `json:"favorited"`
}

type EventSocketFileTrash struct {
	UUID string `json:"uuid"`
}

type EventSocketFileArchived struct {
	UUID string `json:"uuid"`
}

type EventSocketFolderRename struct {
	Name NameStruct `json:"name"`
	UUID string     `json:"uuid"`
}

type EventSocketFolderTrash struct {
	Parent string `json:"parent"`
	UUID   string `json:"uuid"`
}

type EventSocketFolderMove struct {
	Name      NameStruct `json:"name"`
	UUID      string     `json:"uuid"`
	Parent    string     `json:"parent"`
	Timestamp int64      `json:"timestamp"`
	Favorited int        `json:"favorited"`
}

type EventSocketFolderSubCreated struct {
	Name      NameStruct `json:"name"`
	UUID      string     `json:"uuid"`
	Parent    string     `json:"parent"`
	Timestamp int64      `json:"timestamp"`
	Favorited int        `json:"favorited"`
}

type EventSocketFolderRestore struct {
	Name      NameStruct `json:"name"`
	UUID      string     `json:"uuid"`
	Parent    string     `json:"parent"`
	Timestamp int64      `json:"timestamp"`
	Favorited int        `json:"favorited"`
}

type EventSocketFolderColorChanged struct {
	UUID  string `json:"uuid"`
	Color int    `json:"color"`
}

type EventSocketFileDeletedPermanent struct {
	UUID string `json:"uuid"`
}

type NameStruct struct {
	Name string `json:"name"`
}
