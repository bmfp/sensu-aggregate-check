package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/sensu/sensu-go/types"
	"github.com/spf13/cobra"
)

var (
	checkLabels  string
	entityLabels string
	namespaces   string
	apiHost      string
	apiPort      string
	apiUser      string
	apiPass      string
)

type Auth struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"`
}

func main() {
	rootCmd := configureRootCommand()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func configureRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sensu-aggregate-check",
		Short: "The Sensu Go Event Aggregates Check plugin",
		RunE:  run,
	}

	cmd.Flags().StringVarP(&checkLabels,
		"check-labels",
		"c",
		"",
		"aggregate=foo,app=bar")

	cmd.Flags().StringVarP(&entityLabels,
		"entity-labels",
		"e",
		"",
		"aggregate=foo,app=bar")

	cmd.Flags().StringVarP(&namespaces,
		"namespaces",
		"n",
		"default",
		"us-east-1,us-west-2")

	cmd.Flags().StringVarP(&apiHost,
		"api-host",
		"H",
		"127.0.0.1",
		"sensu-backend.example.com")

	cmd.Flags().StringVarP(&apiPort,
		"api-port",
		"p",
		"8080",
		"5555")

	cmd.Flags().StringVarP(&apiUser,
		"api-user",
		"u",
		"admin",
		"ackbar")

	cmd.Flags().StringVarP(&apiPass,
		"api-pass",
		"P",
		"P@ssw0rd!",
		"itsatrap")

	_ = cmd.MarkFlagRequired("check-labels")

	return cmd
}

func run(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		_ = cmd.Help()
		return fmt.Errorf("invalid argument(s) received")
	}

	return evalAggregate()
}

func authenticate() (Auth, error) {
	var auth Auth
	req, err := http.NewRequest(
		"GET",
		fmt.Sprintf("http://%s:%s/auth", apiHost, apiPort),
		nil,
	)
	if err != nil {
		return auth, err
	}

	req.SetBasicAuth(apiUser, apiPass)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return auth, err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return auth, err
	}

	err = json.NewDecoder(bytes.NewReader(body)).Decode(&auth)

	return auth, err
}

func parseLabelArg(labelArg string) map[string]string {
	labels := map[string]string{}

	pairs := strings.Split(labelArg, ",")

	for _, pair := range pairs {
		parts := strings.Split(pair, "=")
		if len(parts) == 2 {
			labels[parts[0]] = parts[1]
		}
	}

	return labels
}

func filterEvents(events []*types.Event) []*types.Event {
	result := []*types.Event{}

	cLabels := parseLabelArg(checkLabels)
	eLabels := parseLabelArg(entityLabels)

	for _, event := range events {
		selected := true

		for key, value := range cLabels {
			if event.Check.ObjectMeta.Labels[key] != value {
				selected = false
			}
		}

		for key, value := range eLabels {
			if event.Entity.ObjectMeta.Labels[key] != value {
				selected = false
			}
		}

		if selected {
			result = append(result, event)
		}
	}

	return result
}

func getEvents(auth Auth, namespace string) ([]*types.Event, error) {
	url := fmt.Sprintf("http://%s:%s/api/core/v2/namespaces/%s/events", apiHost, apiPort, namespace)
	events := []*types.Event{}

	req, err := http.NewRequest("GET", url, nil)

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", auth.AccessToken))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return events, err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	err = json.Unmarshal(body, &events)

	result := filterEvents(events)

	return result, err
}

func evalAggregate() error {
	auth, err := authenticate()

	if err != nil {
		return err
	}

	events, err := getEvents(auth, "default")

	fmt.Printf("hello world: %s\n", events)

	return err
}