/*******************************************************************************
 * Copyright (c) 2019 Red Hat Inc
 *
 * See the NOTICE file(s) distributed with this work for additional
 * information regarding copyright ownership.
 *
 * This program and the accompanying materials are made available under the
 * terms of the Eclipse Public License 2.0 which is available at
 * http://www.eclipse.org/legal/epl-2.0
 *
 * SPDX-License-Identifier: EPL-2.0
 *******************************************************************************/

package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"time"

	"github.com/spf13/cobra"
	"pack.ag/amqp"
)

var insecure bool

func consume(messageType string, uri string, tenant string) error {

	fmt.Printf("Consuming %s from %s ...", messageType, uri)
	fmt.Println()

	var opts = []amqp.ConnOption{}
	if insecure {
		var tls = &tls.Config{
			InsecureSkipVerify: true,
		}
		opts = append(opts, amqp.ConnTLSConfig(tls))
	}

	client, err := amqp.Dial(uri, opts...)
	if err != nil {
		return err
	}

	defer client.Close()

	var ctx = context.Background()

	session, err := client.NewSession()
	if err != nil {
		return err
	}

	defer session.Close(ctx)

	receiver, err := session.NewReceiver(
		amqp.LinkSourceAddress(fmt.Sprintf("%s/%s", messageType, tenant)),
		amqp.LinkCredit(10),
	)
	if err != nil {
		return err
	}
	defer func() {
		ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
		receiver.Close(ctx)
		cancel()
	}()

	fmt.Printf("Consumer running, press Ctrl+C to stop...")

	for {
		// Receive next message
		msg, err := receiver.Receive(ctx)
		if err != nil {
			return err
		}

		// Accept message
		if err := msg.Accept(); err != nil {
			return nil
		}

		fmt.Printf("Message received: %s\n", msg.GetData())
	}
}

func main() {

	var cmdConsume = &cobra.Command{
		Use:   "consume [message endpoint]",
		Short: "Consume and print messages",
		Long:  `Consume messages from the endpoint and print it on the console.`,
		Args:  cobra.MinimumNArgs(3),
		Run: func(cmd *cobra.Command, args []string) {
			if err := consume(args[0], args[1], args[2]); err != nil {
				log.Fatal("Failed to consume messages: ", err)
			}
		},
	}

	var rootCmd = &cobra.Command{Use: "hot"}
	rootCmd.AddCommand(cmdConsume)

	cmdConsume.Flags().BoolVar(&insecure, "insecure", false, "Skip TLS validation")

	if err := rootCmd.Execute(); err != nil {
		panic(err)
	}
}
