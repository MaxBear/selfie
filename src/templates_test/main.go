package main

import (
	"fmt"
	"html/template"
	"log"
	"mns/commons/trunk/db"
	"mns/commons/trunk/misc"
	"mns/selfie/branches/1.1.0/lib"
	"net/http"
	"time"
)

func serveStatic(w http.ResponseWriter, r *http.Request) {
	homedir, _ := misc.GetHomeDir()
	fileName := fmt.Sprintf("%s/go/src/mns/selfie/branches/1.1.0/templates/selfie.html", homedir)
	t, err := template.ParseFiles(fileName)
	if err != nil {
		return
	}

	SrcAddr := dbutils.JsonNullString{}
	SrcAddr.Valid = true
	SrcAddr.String = "sip:nancy.osl@mns.vc"

	DstAddr := dbutils.JsonNullString{}
	DstAddr.Valid = true
	DstAddr.String = "sip:nancy.dev@selfie.vc"

	selfie := Rendezvous.Session{
		StartUtc: dbutils.JsonNullTime{
			Valid: true,
			Time:  time.Now(),
		},
		Duration: 5,
		SrcUri:   SrcAddr,
		DstUri:   DstAddr,
		Tx: Rendezvous.Bw{
			Bw: 6000,
		},
		Rx: Rendezvous.Bw{
			Bw: 6000,
		},
		Medialist: []Rendezvous.Media{
			Rendezvous.Media{
				Type: "video",
				Tx: Rendezvous.RtcpReport{
					Pkt:       265,
					Jitter:    766,
					LostRatio: 0,
				},
				Rx: Rendezvous.RtcpReport{
					Pkt:       128,
					Jitter:    2422,
					LostRatio: 0,
				},
			},
			Rendezvous.Media{
				Type: "audio",
				Tx: Rendezvous.RtcpReport{
					Pkt:       244,
					Jitter:    11000,
					LostRatio: 0,
				},
				Rx: Rendezvous.RtcpReport{
					Pkt:       250,
					Jitter:    2877,
					LostRatio: 0,
				},
			},
			Rendezvous.Media{
				Type: "video",
				Tx: Rendezvous.RtcpReport{
					Pkt:       0,
					Jitter:    0,
					Lost:      0,
					LostRatio: 0,
				},
				Rx: Rendezvous.RtcpReport{
					Pkt:       0,
					Jitter:    0,
					LostRatio: 0,
				},
			},
		},
	}

	data := struct {
		Webhost string
		Selfie  Rendezvous.Session
		Rule    Rendezvous.Alert
	}{
		Webhost: "https://my.selfie.vc",
		Selfie:  selfie,
		Rule: Rendezvous.Alert{
			VideoRxLost:   0,
			VideoTxLost:   0,
			AudioRxLost:   0,
			AudioTxLost:   0,
			ContentRxLost: 0,
			ContentTxLost: 0,
		},
	}

	if err = t.Execute(w, data); err != nil {
		return
	}
	return
}

func main() {
	http.HandleFunc("/", serveStatic)

	log.Println("Listening on :3000...")
	err := http.ListenAndServe(":3000", nil)
	if err != nil {
		log.Fatal(err)
	}
}
