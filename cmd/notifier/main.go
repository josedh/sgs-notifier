package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"sgs-notifier/models"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	log "github.com/sirupsen/logrus"
)

func init() {
	log.SetLevel(log.InfoLevel)
	if _, exists := os.LookupEnv("DEV"); exists {
		log.SetLevel(log.DebugLevel)
	}
}

func main() {
	var (
		err error
		dbx *sqlx.DB
	)
	// Make sure we can connect
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dbx, err = sqlx.ConnectContext(ctx, "postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("Failed to set up postgres conn: %v", err)
	}

	// new contact loop
	//tickChan := time.Tick(3 * time.Hour)
	tickChan := time.Tick(3 * time.Second)
	for range tickChan {
		loc, err := time.LoadLocation("America/New_York")
		if err != nil {
			log.Errorf("Failed to load EST location for date data, please contact webmaster")
		}
		currTime := time.Now()
		start := time.Date(currTime.Year(), currTime.Month(), currTime.Day(), 9, 00, 00, 00, loc)
		end := time.Date(currTime.Year(), currTime.Month(), currTime.Day(), 15, 00, 00, 00, loc)
		if currTime.Before(start) ||
			currTime.After(end) ||
			//currTime.Weekday() == time.Saturday ||
			currTime.Weekday() == time.Sunday {
			// if outside work hours, don't message
			log.Warn("Outside of work hours, skipping notifications")
			continue
		}
		log.Infof("Checking sgs.com contacts table at: %v", time.Now().String())
		if err := checkContacts(dbx); err != nil {
			// there was an error checking for new contacts, log and report
			log.Errorf("Failed to check postgres for new contacts on sgs.com: %v", err)
		}
	}
}

func checkContacts(dbx *sqlx.DB) error {
	var contacts []models.Contact
	var q = `SELECT
				id, name, email, phone, message, captcha_score, acknowledged, created_on, updated_on
			FROM
				contacts
			WHERE
				acknowledged = false`
	if err := dbx.Select(&contacts, q); err != nil {
		log.Debug(err)
		return err
	}
	twilioSID := os.Getenv("TWILIO_ACCOUNT_SID")
	twilioAuth := os.Getenv("TWILIO_AUTH_TOKEN")
	if twilioSID == "" || twilioAuth == "" {
		return fmt.Errorf("Invalid twilio credentials, please check those on the server env and try again")
	}
	for _, c := range contacts {
		log.Infof("Contact %s is unacknowledged, notifying...", c.Name)
		if err := sendToPOC(c, twilioSID, twilioAuth); err != nil {
			// An error occurred sending contact info to sgs admins. Log it
			log.Errorf("Failed to send contact %v to POC: %v", c.String(), err)
		}
		time.Sleep(15 * time.Second) // Give it some time before sending next contact
	}
	log.Infof("Done sending contacts to sgs owner, returning to idle loop")
	return nil
}

func sendToPOC(c models.Contact, sid, auth string) error {
	var (
		urlStr = "https://api.twilio.com/2010-04-01/Accounts/" + sid + "/Messages.json"
		client = &http.Client{}
		err    error
	)
	// Format the message to send to sgs admins
	msg := formatMessage(c)

	// Set up the request
	req, _ := http.NewRequest("POST", urlStr, &msg)
	req.SetBasicAuth(sid, auth)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	log.Infof("REQUEST: %v", req)

	return nil
	// Send it!
	resp, _ := client.Do(req)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var data map[string]interface{}
		decoder := json.NewDecoder(resp.Body)
		if e := decoder.Decode(&data); e != nil {
			log.Debugf("Failed to parse response after sending message: %v", e)
		}
		log.Debugf("Response from sent message: %v", data)
	} else {
		err = fmt.Errorf("Failed to send message to contact. Issue: %v", resp.Status)
	}
	return err
}

func formatMessage(c models.Contact) strings.Reader {
	var msgToPOC = "We are being contacted by '%s' with email: '%s' and phone number '%s'" +
		"for the following reason: '%s'.\n" +
		"Please acknowledged receipt of this contact by replying '%s' to this message."
	msgData := url.Values{}
	msgData.Set("From", os.Getenv("TWILIO_FROM_NUMBER"))
	msgData.Set("To", os.Getenv("TWILIO_TO_NUMBER"))
	msgData.Set("provideFeedback", "true")
	msgData.Set("Body", fmt.Sprintf(msgToPOC, c.Name, c.Email, c.Phone, c.Message, c.ID))
	return *strings.NewReader(msgData.Encode())
}
