package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	"log"
	"mns/selfie/branches/1.1.0/lib"
	"net"
	"net/http"
	"os"
	"strconv"
)

type Selfie struct {
	DbSelfie      *sqlx.DB
	Sessions      chan *Rendezvous.Session
	RunMode       string
	RunTestConfig *Rendezvous.AlertTest
}

func (selfie *Selfie) handler(w http.ResponseWriter, r *http.Request) {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		log.Printf("Error get remote ip address: %s", err.Error)
		return
	}

	userIP := net.ParseIP(ip)
	if userIP == nil {
		log.Printf("userip: %q is not IP:port", r.RemoteAddr)
		return
	}

	sess, err := Rendezvous.Parse(selfie.DbSelfie, ip, r.Body, selfie.RunTestConfig)
	if err == nil {
		selfie.Sessions <- sess
	}
}

func (selfie *Selfie) getRendSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := Rendezvous.GetSessions(selfie.DbSelfie)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else {
		json.NewEncoder(w).Encode(sessions)
	}
}

func (selfie *Selfie) getRendSessionMedias(w http.ResponseWriter, r *http.Request) {
	sessions, err := Rendezvous.GetSessionMedias(selfie.DbSelfie)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else {
		json.NewEncoder(w).Encode(sessions)
	}
}

func main() {
	env := os.Getenv("DEPLOY_TYPE")
	mode := os.Getenv("RUN_MODE")
	DbUser := os.Getenv("SELFIE_DB_USER")
	DbPasswd := os.Getenv("SELFIE_DB_PASSWD")
	DbHost := os.Getenv("SELFIE_DB_HOST")
	DbName := "selfie"
	if env != "prod" {
		DbName += fmt.Sprintf("_%s", env)
	}

	logfname := fmt.Sprintf("/var/log/mns/selfie/selfie.backend.%s.log", env)
	f, err := os.OpenFile(logfname, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		log.Printf("Error : %s\n", err.Error())
		os.Exit(1)
	}
	log.SetOutput(f)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	str := fmt.Sprintf("%s:%s@tcp(%s)/%s?parseTime=true", DbUser, DbPasswd, DbHost, DbName)
	dbcon, err := sqlx.Connect("mysql", str)
	if err != nil {
		log.Printf("Error open data base connection: %s\n", err.Error)
		return
	}
	dbcon.SetMaxIdleConns(0)
	defer dbcon.Close()

	selfie := &Selfie{
		DbSelfie: dbcon,
		Sessions: make(chan *Rendezvous.Session),
		RunMode:  mode,
	}

	if selfie.RunMode == "test" {
		video_loss_tx, _ := strconv.ParseFloat(os.Getenv("SELFIE_TEST_VIDEO_LOSS_TX"), 32)
		video_loss_rx, _ := strconv.ParseFloat(os.Getenv("SELFIE_TEST_VIDEO_LOSS_RX"), 32)
		audio_loss_tx, _ := strconv.ParseFloat(os.Getenv("SELFIE_TEST_AUDIO_LOSS_TX"), 32)
		audio_loss_rx, _ := strconv.ParseFloat(os.Getenv("SELFIE_TEST_AUDIO_LOSS_RX"), 32)
		content_loss_rx, _ := strconv.ParseFloat(os.Getenv("SELFIE_TEST_CONTENT_LOSS_RX"), 32)

		selfie.RunTestConfig = &Rendezvous.AlertTest{
			TestVideoTxLoss:   video_loss_tx,
			TestVideoRxLoss:   video_loss_rx,
			TestAudioTxLoss:   audio_loss_tx,
			TestAudioRxLoss:   audio_loss_rx,
			TestContentRxLoss: content_loss_rx,
		}
	} else {
		selfie.RunTestConfig = nil
	}

	router := mux.NewRouter()
	router.HandleFunc("/callog", selfie.handler).Methods("POST")
	router.HandleFunc("/selfie/api/callogs", selfie.getRendSessions).Methods("GET")
	router.HandleFunc("/selfie/api/callogs/medias", selfie.getRendSessionMedias).Methods("GET")

	SenderEmail := os.Getenv("NOTIFIER_EMAIL_ADDR")
	bccnoc := os.Getenv("BCC_NOC") == "yes"

	go func(selfie *Selfie) {
		for {
			select {
			case session := <-selfie.Sessions:
				log.Printf("Notifier received session: %s", session.Id)
				err := Rendezvous.CheckAlerts(selfie.DbSelfie, session, SenderEmail, bccnoc)
				if err != nil {
					log.Printf("Notifier error: %s", err.Error())
				}
			}
		}
	}(selfie)

	log.Printf("### Service starts...\n")
	panic(http.ListenAndServe(":8888", router))
}
