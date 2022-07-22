package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	// to decode images for thumbnail
	_ "image/png"

	"github.com/ehazlett/flow/pkg/workflows"
	"github.com/sirupsen/logrus"
)

type Processor struct {
	discordName       string
	discordWebhookURL string
}

type Author struct {
	Name string `json:"name"`
}

type Field struct {
	Name   string `json:"name,omitempty"`
	Value  string `json:"value,omitempty"`
	Inline bool   `json:"inline,omitempty"`
}

type Thumbnail struct {
	URL string `json:"url,omitempty"`
}

type Footer struct {
	Text    string `json:"text,omitempty"`
	IconURL string `json:"icon_url,omitempty"`
}

type Embed struct {
	Author    *Author   `json:"author,omitempty"`
	Title     string    `json:"title,omitempty"`
	Color     string    `json:"color,omitempty"`
	Fields    []Field   `json:"fields,omitempty"`
	Thumbnail Thumbnail `json:"thumbnail,omitempty"`
}

type Message struct {
	Username string  `json:"username"`
	Content  string  `json:"content,omitempty"`
	Embeds   []Embed `json:"embeds,omitempty"`
	Footer   Footer  `json:"footer,omitempty"`
}

func (p *Processor) Process(ctx context.Context, cfg *workflows.ProcessorConfig) (*workflows.ProcessorOutput, error) {
	//w := cfg.Workflow

	logrus.Debugf("processor config: %+v", cfg)

	m := &Message{
		Username: p.discordName,
		Footer: Footer{
			Text: "Sent by Flow (flow.ehazlett.dev)",
		},
	}

	for _, iw := range cfg.InputWorkflows {
		if iw.Output != nil {
			m.Content = fmt.Sprintf("Workflow %s Complete", iw.ID)
			params := []string{}
			for k, v := range iw.Parameters {
				params = append(params, fmt.Sprintf("%s=%s", k, v))
			}
			m.Embeds = append(m.Embeds, Embed{
				Fields: []Field{
					{
						Name:  "Name",
						Value: iw.Name,
					},
					{
						Name:  "Params",
						Value: strings.Join(params, ", "),
					},
					{
						Name:  "CreatedAt",
						Value: iw.CreatedAt.String(),
					},
					{
						Name:  "FinishedAt",
						Value: iw.Output.FinishedAt.String(),
					},
					{
						Name:  "Priority",
						Value: iw.Priority.String(),
					},
					{
						Name:  "Duration",
						Value: iw.Output.Duration.String(),
					},
				},
			})
		}

	}

	data, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}

	logrus.Debugf("payload: %s", string(data))

	req, err := http.NewRequest("POST", p.discordWebhookURL, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode > 300 {
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("error sending discord notification: %s", string(b))
	}

	return &workflows.ProcessorOutput{
		FinishedAt: time.Now(),
		Log:        "sent notification to discord",
	}, nil
}
