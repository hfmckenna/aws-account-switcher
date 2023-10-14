package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
)

func main() {
	arg := os.Args[1]
	fmt.Println(arg)
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatal(err)
	}
	stsClient := sts.NewFromConfig(cfg)
	role := fmt.Sprintf("arn:aws:iam::%s:role/PowerUserRole", arg)
	provider := stscreds.NewAssumeRoleProvider(stsClient, role)
	cfg.Credentials = aws.NewCredentialsCache(provider)
	creds, err := cfg.Credentials.Retrieve(context.TODO())
	if err != nil {
		log.Fatal(err)
	}
	type UrlCreds struct {
		SessionId    string `json:"sessionId"`
		SessionKey   string `json:"sessionKey"`
		SessionToken string `json:"sessionToken"`
	}
	urlCreds := UrlCreds{creds.AccessKeyID, creds.SecretAccessKey, creds.SessionToken}
	jsonCreds, err := json.Marshal(urlCreds)
	if err != nil {
		log.Fatal(err)
	}
	jsonString := string(jsonCreds)
	escape := url.QueryEscape(jsonString)
	query := fmt.Sprintf("https://signin.aws.amazon.com/federation?Action=getSigninToken&DurationSeconds=43200&Session=%s", escape)
	resp, err := http.Get(query)
	if err != nil {
		log.Fatal(err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(resp.Body)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	type SignIn struct {
		SigninToken string
	}
	var signIn SignIn
	err = json.Unmarshal(body, &signIn)
	if err != nil {
		log.Fatal(err)
	}
	loginUrl := fmt.Sprintf("https://signin.aws.amazon.com/federation?Action=login&Destination=https://console.aws.amazon.com/&SigninToken=%s", signIn.SigninToken)
	err = open(loginUrl)
	if err != nil {
		log.Fatal(err)
	}
}

func open(url string) error {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
	}
	args = append(args, url)
	return exec.Command(cmd, args...).Start()
}
