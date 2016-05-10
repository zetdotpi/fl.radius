package main

import (
	"database/sql"
	"fmt"
	"github.com/bronze1man/radius"
	_ "github.com/lib/pq"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type radiusService struct{}

func (p radiusService) RadiusHandle(request *radius.Packet) *radius.Packet {
	// a pretty print of the request.
	fmt.Printf("[Authenticate] %s\n", request.String())
	npac := request.Reply()
	switch request.Code {
	case radius.AccessRequest:
		hotspotName := request.GetNASIdentifier()
		username := request.GetUsername()
		mac := username[2:]
		password := request.GetPassword()
		calledStationId := request.GetCalledStationId()
		fmt.Printf("hotspotName %v, username %v, password %v, calledStationId %v\n", hotspotName, username, password, calledStationId)
		row := database.QueryRow("SELECT * FROM hs_mac_phone_pair WHERE mac=$1", mac)

		var (
			rec_mac         string
			rec_phone       string
			rec_valid_until time.Time
		)

		sqlerr = nil
		sqlerr = row.Scan(&rec_mac, &rec_phone, &rec_valid_until)
		if sqlerr != nil {
			fmt.Print("Fuck ya, sql error\n")
			fmt.Print("=================\n")
			fmt.Print(sqlerr)
			npac.Code = radius.AccessReject
		} else if time.Now().Before(rec_valid_until) {
			fmt.Print("Here you go")
			npac.Code = radius.AccessAccept
			// TODO: add session duration and idle timeout
			// TODO: Add Login record to database yo!
		} else {
			fmt.Print("No way, your token is expired")
			// Delete record and reject
			npac.Code = radius.AccessReject
			npac.AVPs = append(npac.AVPs, radius.AVP{Type: radius.ReplyMessage, Value: []byte("No way for you, inglorious scum!")})
		}
		return npac

	case radius.AccountingRequest:
		// accounting start or end
		npac.Code = radius.AccountingResponse
		return npac
	default:
		npac.Code = radius.AccessReject
		return npac
	}
}

var (
	database *sql.DB
	sqlerr   error
)

func main() {
	database, sqlerr = sql.Open("postgres", "host=213.129.63.88 user=feedlikes dbname=feedlikes password='it is a secure password' sslmode=disable")
	if sqlerr != nil {
		log.Print("Error connecting to database")
		panic(sqlerr)
	}
	s := radius.NewServer(":1812", "secret", radiusService{})

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	errChan := make(chan error)
	go func() {
		fmt.Println("waiting for packets...")
		err := s.ListenAndServe()
		if err != nil {
			errChan <- err
		}
	}()
	select {
	case <-signalChan:
		log.Println("stopping server...")
		s.Stop()
	case err := <-errChan:
		log.Println("[ERR] %v", err.Error())
	}
}
