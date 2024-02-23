package EdgeGPT

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Xpl0itU/EdgeGPT-Go/config"
	"github.com/Xpl0itU/EdgeGPT-Go/internal/Logger"
	"github.com/Xpl0itU/EdgeGPT-Go/responses"
)

var log = Logger.NewLogger("GPT Service")

const (
	StyleCreative = "h3imaginative,clgalileo,gencontentv3"
	StyleBalanced = "galileo"
	StylePrecise  = "h3precise,clgalileo"
	DelimiterByte = uint8(30)
	Delimiter     = "\x1e"
)

type GPT struct {
	Config       *config.GPT
	client       *http.Client
	cookies      []*http.Cookie
	Conversation *Conversation
	ExpiredAt    time.Time
	Hub          *Hub
}

// NewGPT create new service
func NewGPT(conf *config.GPT) (*GPT, error) {
	gpt := &GPT{
		Config:    conf,
		ExpiredAt: time.Now().Add(time.Minute * 120),
		client: &http.Client{
			Timeout: conf.TimeoutRequest,
		},
	}

	if err := gpt.createConversation(); err != nil {
		return nil, err
	}

	hub, err := NewHub(gpt.Conversation, conf)
	if err != nil {
		return nil, err
	}
	gpt.Hub = hub

	return gpt, nil
}

// createConversation request for getting new dialog
func (g *GPT) createConversation() error {
	req, err := http.NewRequest("GET", g.Config.ConversationUrl.String(), nil)

	for k, v := range g.Config.Headers {
		req.Header.Set(k, v)
	}

	if err != nil {
		return err
	}

	for _, cookie := range g.cookies {
		req.AddCookie(cookie)
	}

	resp, err := g.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status code not ok: %d, %s", resp.StatusCode, resp.Status)
	}

	r, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	conversation := new(Conversation)
	if err := json.Unmarshal(r, conversation); err != nil {
		return err
	}

	if conversation.Result.Value.ValueOrZero() != "Success" {
		return nil
	}

	g.Conversation = conversation

	log.Infoln("New conversation", conversation)

	return nil
}

/*
AskAsync getting answer async:
Example:

	gpt, err := EdgeGPT.NewGPT(conf) //create service
	if err != nil {
		log.Fatalln(err)
	}

	mw, err := gpt.AskAsync("Привет, ты живой?") // send ask to gpt
	if err != nil {
		log.Fatalln(err)
	}

	go mw.Worker() // Run reading websocket messages

	for _ = range mw.Chan {
		// update answer
		log.Println(mw.Answer.GetAnswer())
	}
*/
func (g *GPT) AskAsync(style, message string) (*responses.MessageWrapper, error) {

	if len(message) > 2000 {
		return nil, fmt.Errorf("message very long, max: %d", 2000)
	}

	log.Infoln("New ask:", message)
	return g.Hub.send(style, message)
}

// AskSync getting answer sync
func (g *GPT) AskSync(style, message string) (*responses.MessageWrapper, error) {
	if len(message) > 2000 {
		return nil, fmt.Errorf("message very long, max: %d", 2000)
	}

	m, err := g.Hub.send(style, message)
	if err != nil {
		return nil, err
	}

	go m.Worker()

	for range m.Chan {
		if m.Final {
			break
		}
	}

	log.Infoln("New ask:", message)
	return m, nil
}
