package Rendezvous

import (
	"errors"
	"fmt"
	"github.com/jmoiron/sqlx"
	"log"
	"mns/commons/trunk/misc"
	"mns/nms/branches/cr_components_nagios/mailer"
	"strings"
)

type SessionMedia struct {
	Session
	Media
}

type Alert struct {
	Id            int     `db:"alert_id"`
	Name          string  `db:"name"`
	CompanyId     int     `db:"company_id"`
	UserId        int     `db:"user_id"`
	SrcAddr       string  `db:"src_addr"`
	DstAddr       string  `db:"dst_addr"`
	VideoRxLost   float64 `db:"video_rx_lost"`
	VideoTxLost   float64 `db:"video_tx_lost"`
	AudioRxLost   float64 `db:"audio_rx_lost"`
	AudioTxLost   float64 `db:"audio_tx_lost"`
	ContentRxLost float64 `db:"content_rx_lost"`
	ContentTxLost float64 `db:"content_tx_lost"`
	DstEmails     string  `db:"dst_email"`
	DstEmailsArr  []string
}

type AlertWithUser struct {
	Alert
	UserEmail string `db:"email"`
}

type AlertTest struct {
	TestVideoTxLoss   float64
	TestVideoRxLoss   float64
	TestAudioTxLoss   float64
	TestAudioRxLoss   float64
	TestContentRxLoss float64
}

func GetAlerts(db *sqlx.DB, CompanyId int) (alerts []AlertWithUser, err error) {
	alerts = make([]AlertWithUser, 0)
	sql := fmt.Sprintf("select a.*, u.email from alerts as a join user as u on a.user_id=u.id where a.company_id=%d", CompanyId)
	rows, err := db.Queryx(sql)
	if err != nil {
		log.Printf("error: %s\n", err.Error())
		return
	}
	alert := AlertWithUser{}
	for rows.Next() {
		err = rows.StructScan(&alert)
		alert.DstEmailsArr = misc.GenList(alert.DstEmails)
		alerts = append(alerts, alert)
	}
	return
}

func CheckAlerts(db *sqlx.DB, s *Session, SenderEmail string, bccnoc bool) (err error) {
	if s.CompanyId == 0 {
		err = errors.New(fmt.Sprintf("Unable to find company_id for session %s", s.Id))
		return
	}

	alerts, err := GetAlerts(db, s.CompanyId)
	for _, a := range alerts {
		log.Printf("Notifier check alert id %d userid %d src_addr \"%s\" dst_addr \"%s\"", a.Id, a.UserId, a.SrcAddr, a.DstAddr)
		if a.SrcAddr != "any" && s.SrcUri.Valid && !strings.Contains(s.SrcUri.String, a.SrcAddr) {
			continue
		}
		if a.DstAddr != "any" && s.DstUri.Valid && !strings.Contains(s.DstUri.String, a.DstAddr) {
			continue
		}
		alert := false
		for _, m := range s.Medialist {
			if m.Type == "video" {
				if m.Tx.Pkt == 0 && m.Tx.Lost == 0 && m.Tx.Jitter == 0 {
					log.Printf("%s %f / %f", m.Type, m.Rx.LostRatio, a.ContentRxLost)
					if m.Rx.LostRatio >= a.ContentRxLost {
						alert = true
						log.Printf("\t => alert")
					}
				} else {
					log.Printf("%s %f %f / %f %f", m.Type, m.Tx.LostRatio, m.Rx.LostRatio, a.VideoTxLost, a.VideoRxLost)
					if m.Tx.LostRatio >= a.VideoTxLost || m.Rx.LostRatio >= a.VideoRxLost {
						alert = true
						log.Printf("\t => alert")
					}
				}
			} else {
				log.Printf("%s %f %f / %f %f", m.Type, m.Tx.LostRatio, m.Rx.LostRatio, a.AudioTxLost, a.AudioRxLost)
			}
			if m.Type == "audio" && m.Tx.LostRatio >= a.AudioTxLost || m.Rx.LostRatio >= a.AudioRxLost {
				alert = true
				log.Printf("\t => alert")
			}
		}
		if alert {
			webhost := "https://my.selfie.vc"
			data := struct {
				Webhost string
				Selfie  Session
				Rule    AlertWithUser
				Zero    float64
			}{
				Webhost: webhost,
				Selfie:  *s,
				Rule:    a,
				Zero:    float64(0),
			}
			cc := []string{}
			bcc := []string{}
			if bccnoc {
				bcc = append(bcc, "noc@mns.vc")
			}
			r := mailer.NewRequest(SenderEmail, webhost, a.DstEmailsArr, cc, bcc, "[Selfie.vc Alert] session errors")
			homedir, _ := misc.GetHomeDir()
			err = r.Send(fmt.Sprintf("%s/go/src/mns/selfie/branches/1.1.0/templates/selfie_kjell2.html", homedir), data)
			if err != nil {
				log.Printf("Failed to send mail notification, error: %s\n", err)
			} else {
				log.Printf("Successfully send mail notification to %s", strings.Join(a.DstEmailsArr[:], ","))
			}
		}
	}
	return
}
