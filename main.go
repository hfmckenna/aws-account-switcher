package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/manifoldco/promptui"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
)

func main() {
	accountsFile, err := os.Open("account-roles.json")
	defer func(accounts *os.File) {
		err := accounts.Close()
		if err != nil {

		}
	}(accountsFile)
	configDecoder := json.NewDecoder(accountsFile)
	accountsConfig := map[string]interface{}{}
	err = configDecoder.Decode(&accountsConfig)
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatal(err)
	}
	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		return
	}
	accountKeys := make([]string, 0, len(accountsConfig))
	for k := range accountsConfig {
		accountKeys = append(accountKeys, k)
	}
	prompt := promptui.Select{
		Label: "Select Account:",
		Items: accountKeys,
	}
	_, result, err := prompt.Run()
	account := accountsConfig[result].(map[string]interface{})
	rolePrompt := promptui.Select{
		Label: "Select Environment:",
		Items: []string{"staging", "production"},
	}
	_, roleSelect, err := rolePrompt.Run()
	selectedRole := account[roleSelect].(map[string]interface{})
	stsClient := sts.NewFromConfig(cfg)
	roleArn := fmt.Sprintf("arn:aws:iam::%s:role/%s", selectedRole["id"], selectedRole["role"])
	provider := stscreds.NewAssumeRoleProvider(stsClient, roleArn)
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
	println(loginUrl)
	err = open(loginUrl)
	if err != nil {
		log.Fatal(err)
	}
}

func open(url string) error {
	var cmd string
	var urlArg string
	switch runtime.GOOS {
	case "windows":
		cmd = "powershell"
		urlArg = fmt.Sprintf("-c 'Start %s'", url)
	case "darwin":
		cmd = "open"
		urlArg = url
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
		urlArg = url
	}
	return exec.Command(cmd, urlArg).Start()
}
