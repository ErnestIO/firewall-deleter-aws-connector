/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/nats-io/nats"
)

var nc *nats.Conn
var natsErr error

func processEvent(data []byte) (*Event, error) {
	var ev Event
	err := json.Unmarshal(data, &ev)
	return &ev, err
}

func eventHandler(m *nats.Msg) {
	f, err := processEvent(m.Data)
	if err != nil {
		nc.Publish("firewall.delete.aws.error", m.Data)
		return
	}

	if err = f.Validate(); err != nil {
		f.Error(errors.New("Security Group is invalid"))
		return
	}

	err = deleteFirewall(f)
	if err != nil {
		f.Error(err)
		return
	}

	f.Complete()
}

func deleteFirewall(ev *Event) error {
	creds := credentials.NewStaticCredentials(ev.DatacenterAccessKey, ev.DatacenterAccessToken, "")
	svc := ec2.New(session.New(), &aws.Config{
		Region:      aws.String(ev.DatacenterRegion),
		Credentials: creds,
	})

	req := ec2.DeleteSecurityGroupInput{
		GroupId: aws.String(ev.SecurityGroupAWSID),
	}

	_, err := svc.DeleteSecurityGroup(&req)
	if err != nil {
		return err
	}

	return nil
}

func main() {
	natsURI := os.Getenv("NATS_URI")
	if natsURI == "" {
		natsURI = nats.DefaultURL
	}

	nc, natsErr = nats.Connect(natsURI)
	if natsErr != nil {
		log.Fatal(natsErr)
	}

	fmt.Println("listening for firewall.delete.aws")
	nc.Subscribe("firewall.delete.aws", eventHandler)

	runtime.Goexit()
}
