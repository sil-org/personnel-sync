package alert

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
	"github.com/getsentry/sentry-go"
)

type Config struct {
	AWSRegion          string
	CharSet            string
	ReturnToAddr       string
	SubjectText        string
	RecipientEmails    []string
	AWSAccessKeyID     string
	AWSSecretAccessKey string
}

func New(config Config, err error) {
	sentry.CaptureException(err)
	sendEmail(config, err.Error())
}

func sendEmail(config Config, body string) {
	charSet := config.CharSet

	subject := config.SubjectText
	subjContent := types.Content{
		Charset: &charSet,
		Data:    &subject,
	}

	msgContent := types.Content{
		Charset: &charSet,
		Data:    &body,
	}

	msgBody := types.Body{
		Text: &msgContent,
	}

	emailMsg := types.Message{
		Subject: &subjContent,
		Body:    &msgBody,
	}

	// Only report the last email error
	lastError := ""
	badRecipients := []string{}

	// Send emails to one recipient at a time to avoid one bad email sabotaging it all
	for _, address := range config.RecipientEmails {
		err := sendAnEmail(emailMsg, address, config)
		if err != nil {
			lastError = err.Error()
			badRecipients = append(badRecipients, address)
		}
	}

	if lastError != "" {
		addresses := strings.Join(badRecipients, ", ")
		log.Printf("Error sending email from '%s' to '%s': %s",
			config.ReturnToAddr, addresses, lastError)
	}
}

func sendAnEmail(emailMsg types.Message, recipient string, cfg Config) error {
	input := &ses.SendEmailInput{
		Destination: &types.Destination{
			ToAddresses: []string{recipient},
		},
		Message: &emailMsg,
		Source:  aws.String(cfg.ReturnToAddr),
	}

	svc, err := createSESService(cfg.AWSRegion, cfg.AWSAccessKeyID, cfg.AWSSecretAccessKey)
	if err != nil {
		return fmt.Errorf("failed to create SES service: %w", err)
	}

	result, err := svc.SendEmail(context.Background(), input)
	if err != nil {
		return fmt.Errorf("error sending email: %w", err)
	}
	log.Printf("alert message sent to %s, message ID: %s", recipient, *result.MessageId)
	return nil
}

func createSESService(region, key, secret string) (*ses.Client, error) {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, fmt.Errorf("AWS SDK LoadDefaultConfig failed: %w", err)
	}

	cfg.Region = region
	if key != "" && secret != "" {
		cfg.Credentials = credentials.NewStaticCredentialsProvider(key, secret, "")
	}

	return ses.NewFromConfig(cfg), nil
}
