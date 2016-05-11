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
		identity := request.GetCalledStationId()
		log.Println("hotspotName = %v, username = %v, password = %v, identity (calledStationId) = %v\n", hotspotName, username, password, identity)

		var (
			rec_mac         string
			rec_phone       string
			rec_valid_until time.Time
		)

		sqlerr = nil

		sqlerr = database.QueryRow("SELECT * FROM hs_mac_phone_pair WHERE mac=$1", mac).Scan(&rec_mac, &rec_phone, &rec_valid_until)
		if sqlerr != nil {
			log.Println("SQL error")
			log.Println("=========")
			log.Println(sqlerr)
			npac.Code = radius.AccessReject
		} else if time.Now().Before(rec_valid_until) {
			log.Println("Login successfull")
			npac.Code = radius.AccessAccept

			npac.AVPs = append(npac.AVPs,
				radius.AVP{Type: radius.SessionTimeout, Value: []byte("10800")}, // 3 hours
				radius.AVP{Type: radius.IdleTimeout, Value: []byte("10800")},    // 3 hours too
			)

			// TODO: Add Login record to database yo!
			var hotspot_id, loginrecord_id int

			sqlerr = database.QueryRow("SELECT id FROM hotspot WHERE identity=$1", identity).Scan(&hotspot_id)
			if sqlerr != nil {
				log.Println("Cannot get hotspot identity")
				log.Println(sqlerr)
				sqlerr = nil
			}
			sqlerr = database.QueryRow("INSERT INTO loginrecord (datetime, method, access_token) VALUES ($1, $2, $3) RETURNING id", time.Now(), "radius", "_").Scan(&loginrecord_id)
			if sqlerr != nil {
				log.Println("Cannot insert loginrecord")
				log.Println(sqlerr)
				sqlerr = nil
			}

			log.Println("LoginRecord number %v", loginrecord_id)

		} else {
			log.Println("Expired token")

			var deleted_id int
			sqlerr = database.QueryRow("DELETE FROM hs_mac_phone_pair WHERE mac=$1", mac).Scan(&deleted_id)
			log.Println("Deleted hs_mac_phone_pair record with id = $1", deleted_id)
			npac.Code = radius.AccessReject
			npac.AVPs = append(npac.AVPs, radius.AVP{Type: radius.ReplyMessage, Value: []byte("The token is invalid, please login")})
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
	database, sqlerr = sql.Open("postgres", "host=213.129.63.88 user=feedlikes dbname=feedlikes_test password='it is a secure password' sslmode=disable")
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

// TODO: move settings to external file
