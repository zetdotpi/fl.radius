package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/bronze1man/radius"
	_ "github.com/lib/pq"

	// "log/syslog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

type radiusService struct{}

func (p radiusService) RadiusHandle(request *radius.Packet) *radius.Packet {
	// a pretty print of the request.
	// log.Printf("[Authenticate] %s\n", request.String())
	npac := request.Reply()
	switch request.Code {
	case radius.AccessRequest:
		username := request.GetUsername()
		mac := username
		identity := request.GetCalledStationId()

		var (
			rec_mac         string
			rec_phone       string
			rec_valid_until time.Time
			rec_validated   bool
		)

		// return Reject-Access by default
		npac.Code = radius.AccessReject

		sqlerr = nil
		sqlerr = database.QueryRow("SELECT * FROM hs_mac_phone_pair WHERE mac=$1", mac).Scan(&rec_mac, &rec_phone, &rec_valid_until, &rec_validated)
		if sqlerr != nil {
			log.Printf("<SQL ERROR>: no pair found with mac = %v. %v\n", mac, sqlerr)
		} else if time.Now().Before(rec_valid_until) && rec_validated {
			// log.Println("Login successfull")
			npac.Code = radius.AccessAccept

			npac.AVPs = append(npac.AVPs,
				radius.AVP{Type: radius.SessionTimeout, Value: []byte("10800")}, // 3 hours
				radius.AVP{Type: radius.IdleTimeout, Value: []byte("10800")},    // 3 hours too
			)

			// Adding Login record to database
			var hotspot_id, loginrecord_id int

			sqlerr = database.QueryRow("SELECT id FROM hotspot WHERE identity=$1", identity).Scan(&hotspot_id)
			if sqlerr != nil {
				log.Printf("<SQL ERROR>: no hotspot %v found. %v\n", identity, sqlerr)
				sqlerr = nil
			}
			sqlerr = database.QueryRow("INSERT INTO loginrecord (datetime, method, access_token, phone, hotspot_id) VALUES ($1, $2, $3, $4, $5) RETURNING id", time.Now(), "radius", "_", rec_phone, hotspot_id).Scan(&loginrecord_id)
			if sqlerr != nil {
				log.Printf("<SQL ERROR>: cannot insert login record. %v\n", sqlerr)
				sqlerr = nil
			}

			log.Printf("LoginRecord number %v\n", loginrecord_id)

		} else {
			if time.Now().After(rec_valid_until) {
				var deleted_id int
				sqlerr = database.QueryRow("DELETE FROM hs_mac_phone_pair WHERE mac=$1", mac).Scan(&deleted_id)
				log.Printf("EXPIRED pair. Deleted hs_mac_phone_pair %v\n", deleted_id)
			}
			npac.Code = radius.AccessReject
			npac.AVPs = append(npac.AVPs, radius.AVP{Type: radius.ReplyMessage, Value: []byte("The token is invalid, please login")})
		}

		if npac.Code == radius.AccessAccept {
			log.Printf("ACCEPT %v @ %v\n", username, identity)
		} else if npac.Code == radius.AccessReject {
			log.Printf("REJECT %v @ %v\n", username, identity)
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

	srvPort   uint16
	srvSecret string

	host    string
	port    uint16
	dbname  string
	dbuser  string
	dbpass  string
	sslmode string = "disable"
)

const (
	helptext = `
Feedlikes.Radius help
=====================

To run this application properly you need to define environment variables:

RAD_DBHOST (optional, defaults to "localhost") - database host
RAD_DBPORT (optional, defaults to 5432) - database port
RAD_DBNAME - database name
RAD_DBUSER - database username
RAD_DBPASS - database password

SRV_PORT (optional, default to 1812) - port, on which radius server itself works
SRV_SECRET - secret for radius server
`
)

func main() {
	for _, arg := range os.Args {
		if arg == "-h" || arg == "--help" {
			log.Print(helptext)
			os.Exit(0)
		}
	}

	// logwriter, e := syslog.New(syslog.LOG_NOTICE, "flRadius")
	// if e == nil {
	// 	log.SetOutput(logwriter)
	// }

	err := readEnv()
	if err != nil {
		log.Print(err)
		os.Exit(0)
	}

	sqlConnectionString := fmt.Sprintf("host=%v port=%v dbname=%v user=%v password='%v' sslmode=%v", host, port, dbname, dbuser, dbpass, sslmode)

	database, sqlerr = sql.Open("postgres", sqlConnectionString)

	if sqlerr != nil {
		log.Print("Error connecting to database")
		panic(sqlerr)
	}
	defer database.Close()
	serverHost := fmt.Sprintf(":%v", srvPort)
	s := radius.NewServer(serverHost, srvSecret, radiusService{})

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	errChan := make(chan error)
	go func() {
		log.Println("waiting for packets...")
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
		log.Printf("[ERR] %v\n", err.Error())
	}
}

func readEnv() error {
	var errDesc string = ""

	srvPortStr := os.Getenv("SRV_PORT")
	if srvPortStr == "" {
		srvPort = 1812
	} else {
		if srvPortNum, err := strconv.ParseUint(srvPortStr, 10, 16); err == nil {
			srvPort = uint16(srvPortNum)
		} else {
			errDesc += "SRV_PORT: " + err.Error() + "\n"
		}
	}

	srvSecret = os.Getenv("RAD_SECRET")
	if srvSecret == "" {
		errDesc += "RAD_SECRET isn't defined.\n"
	}

	host = os.Getenv("RAD_DBHOST")
	if host == "" {
		host = "localhost"
	}

	portStr := os.Getenv("RAD_DBPORT")
	if portStr == "" {
		port = 5432
	} else {
		if portNum, err := strconv.ParseUint(portStr, 10, 16); err == nil {
			port = uint16(portNum)
		} else {
			errDesc += "RAD_DBPORT: " + err.Error() + "\n"
		}
	}

	dbname = os.Getenv("RAD_DBNAME")
	if dbname == "" {
		errDesc += "RAD_DBNAME isn't defined.\n"
	}

	dbuser = os.Getenv("RAD_DBUSER")
	if dbuser == "" {
		errDesc += "RAD_DBUSER isn't defined.\n"
	}

	dbpass = os.Getenv("RAD_DBPASS")
	if dbpass == "" {
		errDesc += "RAD_DBPASS isn't defined.\n"
	}

	if errDesc == "" {
		return nil
	} else {
		return errors.New(errDesc)
	}
}
