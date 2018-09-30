package main

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/bronze1man/radius"
	_ "github.com/lib/pq"
	"log"
	"os"
	"os/signal"
	"strconv"
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

type Config struct {
	SQLAddress string
	SQLPort    string
	DBName     string
	DBUsername string
	DBPass     string
}

func (c Config) readFromEnv() {
	c.os.Getenv("dbhost")
	os.Getenv("dbname")
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

SRV_PORT - port, on which radius server itself works
SRV_SECRET - secret for radius server
`
)

const (
	DB_HOST     = "localhost"
	DB_NAME     = "feedlikes"
	DB_USERNAME = "feedlikes"
	DB_PASSWORD = "it is a secure password"
)

func readConfig() {
}

func main() {
<<<<<<< HEAD
	for _, arg := range os.Args {
		if arg == "-h" || arg == "--help" {
			fmt.Print(helptext)
			os.Exit(0)
		}
	}

	err := readEnv()
	if err != nil {
		fmt.Print(err)
		os.Exit(0)
	}

	sqlConnectionString := fmt.Sprintf("host=%v port=%v dbname=%v user=%v password='%v' sslmode=%v", host, port, dbname, dbuser, dbpass, sslmode)

	database, sqlerr = sql.Open("postgres", sqlConnectionString)

=======
	// database, sqlerr = sql.Open("postgres", "host=213.129.63.88 user=feedlikes dbname=feedlikes_test password='it is a secure password' sslmode=disable")
	database, sqlerr = sql.Open("postgres", "host=feedlikes.ru user=feedlikes dbname=feedlikes_test password='it is a secure password' sslmode=disable")
>>>>>>> 616d09e8dd8c05373867a513a8b095c8a83d711c
	if sqlerr != nil {
		log.Print("Error connecting to database")
		panic(sqlerr)
	}
<<<<<<< HEAD
	serverHost := fmt.Sprintf(":%v", srvPort)
	s := radius.NewServer(serverHost, srvSecret, radiusService{})
=======

	defer database.Close()
	s := radius.NewServer(":1812", "secret", radiusService{})
>>>>>>> 616d09e8dd8c05373867a513a8b095c8a83d711c

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
