package GRPC

import (
	"github.com/Xpl0itU/EdgeGPT-Go"
	pb "github.com/Xpl0itU/EdgeGPT-Go/pkg/GRPC/GPT"
	"google.golang.org/protobuf/types/known/structpb"
	"log"
	"time"
)

type Server struct {
	pb.UnimplementedGptServiceServer
	Storage *EdgeGPT.Storage
}

func NewServer(s *EdgeGPT.Storage) *Server {
	return &Server{
		Storage: s,
	}
}

func (s *Server) Ask(r *pb.AskRequest, stream pb.GptService_AskServer) error {
	gpt, err := s.Storage.GetOrSet(r.GetId())
	if err != nil {
		return err
	}

	message, err := gpt.AskAsync(r.GetStyle(), r.GetText())
	if err != nil {
		return err
	}

	go func() {
		err := message.Worker()
		if err != nil {
			log.Println("Worker err:", err)
		}
	}()

	time.Sleep(time.Second)

	for _ = range message.Chan {
		msg := message.Answer.GetAnswer()
		if msg == "" {
			continue
		}

		res := &pb.AskResponse{
			Text:       message.Answer.GetAnswer(),
			MaxUnit:    uint64(message.Answer.GetMaxUnit()),
			UnitUser:   uint64(message.Answer.GetUserUnit()),
			ExpiryTime: uint64(gpt.ExpiredAt.Unix()),
		}

		suggestions := message.Answer.GetSuggestions()
		if suggestions != nil {
			res.Suggestions = make([]*structpb.Struct, len(suggestions))
			for i, sug := range suggestions {
				res.Suggestions[i], _ = structpb.NewStruct(sug)
			}
		}

		if err := stream.Send(res); err != nil {
			return err
		}
	}

	return nil
}
