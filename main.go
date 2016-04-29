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
	// "time"
)

type radiusService struct{}

func (p radiusService) RadiusHandle(request *radius.Packet) *radius.Packet {
	// a pretty print of the request.
	fmt.Printf("[Authenticate] %s\n", request.String())
	npac := request.Reply()
	switch request.Code {
	case radius.AccessRequest:
		log.Print(request.GetNASIdentifier())
		log.Print(request.GetNasIpAddress())
		log.Print(request.GetUsername())
		log.Print(request.GetPassword())
		log.Print(request.GetCalledStationId())
		// check username and password
		if request.GetUsername() == "a" && request.GetPassword() == "a" {
			npac.Code = radius.AccessAccept
			return npac
		} else {
			npac.Code = radius.AccessReject
			npac.AVPs = append(npac.AVPs, radius.AVP{Type: radius.ReplyMessage, Value: []byte("Your message here")})
			return npac
		}
	case radius.AccountingRequest:
		// accounting start or end
		npac.Code = radius.AccountingResponse
		return npac
	default:
		npac.Code = radius.AccessAccept
		return npac
	}
}

func main() {
	db, err := sql.Open("postgres", "host=213.129.63.88 user=feedlikes dbname=feedlikes password='it is a secure password' sslmode=disable")
	if err != nil {
		log.Print("Error connecting to database")
		panic(err)
	}
	log.Print(db)

	// rows, err := db.Query("SELECT * FROM hs_mac_phone_pair")
	// if err != nil {
	// 	log.Print("Error quering database")
	// 	panic(err)
	// }

	// var (
	// 	mac         string
	// 	phone       string
	// 	valid_until time.Time
	// )
	// log.Print(rows.Columns())

	// for rows.Next() {
	// 	err := rows.Scan(&mac, &phone, &valid_until)

	// 	if err != nil {
	// 		log.Print("Error scanning row")
	// 		log.Print(err)
	// 	}

	// 	log.Print(mac, phone, valid_until)
	// 	log.Print(time.Now().Before(valid_until))
	// }

	// rows.Close()

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
