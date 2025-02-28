package service

import (
	"context"
	messagesv1 "github.com/Vsevololod/tg-api-contracts-lib/gen/go/messages"
	"log/slog"
	"tg-enricher/domain"
	"tg-enricher/domain/models"
	"tg-enricher/lib/logger/sl"

	"go.opentelemetry.io/otel"
)

type VideoUpdater interface {
	UpdateVideo(ctx context.Context,
		path string,
		title string,
		duration int64,
		timestamp int64,
		filesize int64,
		thumbnail string,
		channelUrl string,
		channelID string,
		channel string,
		videoID string,
		hashID string) error
}

type VideoProvider interface {
	GetVideoById(ctx context.Context, videoId string) (models.Video, error)
}

// MessageProcessService — сервис обработки сообщений
type MessageProcessService struct {
	inputMessageChannel  chan domain.InputMessageWithContext
	outputMessageChannel chan domain.OutputMessageWithContext
	videoProvider        VideoProvider
	VideoUpdater         VideoUpdater
	log                  *slog.Logger
}

// NewMessageProcessService создает новый сервис и принимает канал для сообщений
func NewMessageProcessService(
	inputMessageChannel chan domain.InputMessageWithContext,
	outputMessageChannel chan domain.OutputMessageWithContext,
	videoProvider VideoProvider,
	VideoUpdater VideoUpdater,
	log *slog.Logger,
) *MessageProcessService {
	return &MessageProcessService{
		inputMessageChannel:  inputMessageChannel,
		outputMessageChannel: outputMessageChannel,
		videoProvider:        videoProvider,
		VideoUpdater:         VideoUpdater,
		log:                  log,
	}
}

// StartProcessing запускает обработку сообщений в отдельной горутине
func (s *MessageProcessService) StartProcessing(workerCount int) {
	for i := 0; i < workerCount; i++ {
		go func(workerID int) {
			for msg := range s.inputMessageChannel {
				s.ProcessMessage(workerID, msg)
			}
		}(i)
	}
}

// ProcessMessage выполняет обработку сообщения
func (s *MessageProcessService) ProcessMessage(workerID int, msg domain.InputMessageWithContext) {
	s.log.Info("Processing Message with id", slog.String("uuid", msg.UUID), slog.Int64("worker", int64(workerID)))

	tracer := otel.Tracer("tg-dispatcher")
	ctx, span := tracer.Start(msg.Context, "ProcessMessage")
	defer span.End()

	video, err := s.videoProvider.GetVideoById(ctx, msg.UUID)
	if err != nil {
		s.log.Error("Error getting video", sl.Err(err))
		span.RecordError(err)
		return
	}

	message := msg.Message
	err = s.VideoUpdater.UpdateVideo(ctx,
		message.Path,
		message.Title,
		int64(message.Duration),
		message.Timestamp,
		message.Filesize,
		message.Thumbnail,
		message.ChannelUrl,
		message.ChannelId,
		message.Channel,
		message.Id,
		msg.UUID)
	if err != nil {
		s.log.Error("Error updating video", sl.Err(err))
		span.RecordError(err)
		return
	}

	s.outputMessageChannel <- domain.OutputMessageWithContext{
		Message: &messagesv1.TgSendMessage{
			Text:   message.Title,
			UserId: uint64(video.UserID),
			Type:   messagesv1.MessageType_IMAGE,
			Params: map[string]string{
				messagesv1.MessageParams_FILE_URL.String():  message.Path,
				messagesv1.MessageParams_PHOTO_URL.String(): message.Thumbnail,
			},
		},
		UUID:        msg.UUID,
		Destination: "",
		Context:     msg.Context,
	}

}

func (s *MessageProcessService) StopProcessing() {

}
