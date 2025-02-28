package domain

import (
	"context"
	messages "github.com/Vsevololod/tg-api-contracts-lib/gen/go/messages"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	//"google.golang.org/protobuf/proto"
)

type InputMessageWithContext struct {
	Message *messages.VideoDownloadedMessage
	UUID    string
	Context context.Context
}

type OutputMessageWithContext struct {
	Message     *messages.TgSendMessage
	UUID        string
	Destination string
	Context     context.Context
}

func ParseMessage(jsonData []byte, isJson bool) (*messages.VideoDownloadedMessage, error) {
	message := messages.VideoDownloadedMessage{}
	if isJson {
		err := protojson.Unmarshal(jsonData, &message)
		if err != nil {
			return nil, err
		}
	} else {
		err := proto.Unmarshal(jsonData, &message)
		if err != nil {
			return nil, err
		}
	}
	return &message, nil
}
