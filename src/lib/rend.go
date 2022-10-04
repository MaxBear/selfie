package Rendezvous

import (
	"bytes"
	"encoding/xml"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"io"
	"io/ioutil"
	"log"
	"mns/commons/trunk/db"
	"mns/commons/trunk/network"
	"time"
)

type Session struct {
	Id          string                 `xml:"id" db:"Id"`
	Start       int64                  `xml:"start"`
	StartUtc    dbutils.JsonNullTime   `xml:"-" db:"StartUtc"`
	Duration    int                    `xml:"dur" db:"Duration"`
	Dir         int                    `xml:"dir" db:"Dir"`
	Established int                    `xml:"established" db:"Established"`
	RendServer  uint32                 `xml:"-" db:"RendServer"`
	Host        string                 `xml:"host"`
	Uri         string                 `xml:"uri"`
	SrcUri      dbutils.JsonNullString `xml:"-" db:"SrcUri"`
	DstUri      dbutils.JsonNullString `xml:"-" db:"DstUri"`
	DstTag      string                 `xml:"-" db:"DstTag"`
	Name        string                 `xml:"name"`
	Status      string                 `xml:"status" db:"Status"`
	Tx          Bw                     `xml:"tx"`
	Rx          Bw                     `xml:"rx"`
	TxBw        int                    `xml:"-" db:"TxBw"`
	RxBw        int                    `xml:"-" db:"RxBw"`
	Medialist   []Media                `xml:"medialist>media"`
	CompanyId   int
}

type SessionDbWithHost struct {
	//SessionDb
	Session
	Hostname string `db:"Hostname"`
}

type Bw struct {
	Bw int `xml:"bw"`
}

type RtcpReport struct {
	Pkt       int `xml:"pkt"`
	Lost      int `xml:"lost"`
	Jitter    int `xml:"jit"`
	LostRatio float64
}

type Media struct {
	SessionId string     `xml:"-" db:"SessionId"`
	Raddr     string     `xml:"raddr" db:"Raddr"`
	Rport     int        `xml:"rport" db:"Rport"`
	Type      string     `xml:"type" db:"Type"`
	Enabled   int        `xml:"enabled" db:"Enabled"`
	Tx        RtcpReport `xml:"tx"`
	Rx        RtcpReport `xml:"rx"`
	TxPkt     int        `xml:"-" db:"TxPkt"`
	TxLost    int        `xml:"-" db:"TxLost"`
	TxJitter  int        `xml:"-" db:"TxJitter"`
	RxPkt     int        `xml:"-" db:"RxPkt"`
	RxLost    int        `xml:"-" db:"RxLost"`
	RxJitter  int        `xml:"-" db:"RxJitter"`
}

type SelfieHost struct {
	RendServerIp uint32                 `db:"RendServerIp"`
	Tag          string                 `db:"Tag"`
	SipAddr      dbutils.JsonNullString `db:"SipAddr"`
	CompanyId    int                    `db:"company_id"`
}

func MakeNullString(input string) (output dbutils.JsonNullString) {
	output.String = input
	output.Valid = true
	return
}

func GetSelfieUri(db *sqlx.DB, ip uint32, host string) (selfie SelfieHost, err error) {
	sql := fmt.Sprintf("select * from Selfie_Hosts where RendServerIp=%d and Tag=\"%s\"", ip, host)
	rows, err := db.Queryx(sql)
	if err != nil {
		log.Printf("error: %s\n", err.Error())
		return
	}
	selfie = SelfieHost{}
	for rows.Next() {
		err = rows.StructScan(&selfie)
	}
	return
}

func InsertSession(db *sqlx.DB, sess *Session, ip string) (err error) {
	iip := nwutils.Inet_aton(ip)

	selfie, _ := GetSelfieUri(db, iip, sess.Host)

	//dir is short for call direction: INBOUND = 1, OUTBOUND = 2 (from rendezvous perspective)
	sess.StartUtc = dbutils.JsonNullTime{
		Valid: true,
		Time:  time.Unix(sess.Start, 0).UTC(),
	}
	sess.RendServer = iip
	sess.SrcUri = MakeNullString(sess.Uri)
	sess.DstUri = selfie.SipAddr
	sess.CompanyId = selfie.CompanyId
	sess.DstTag = sess.Host
	sess.TxBw = sess.Tx.Bw
	sess.RxBw = sess.Rx.Bw

	params := dbutils.GetFields(*sess, false)
	s1, s2 := dbutils.GenInsert(params)
	q := fmt.Sprintf("insert into Sessions %s values %s", s1, s2)
	_, err = db.NamedExec(q, *sess)
	return
}

func InsertSessionMedia(db *sqlx.DB, media Media, SessionId string) (err error) {
	newmedia := Media{
		SessionId: SessionId,
		Raddr:     media.Raddr,
		Rport:     media.Rport,
		Enabled:   media.Enabled,
		Type:      media.Type,
		TxPkt:     media.Tx.Pkt,
		TxLost:    media.Tx.Lost,
		TxJitter:  media.Tx.Jitter,
		RxPkt:     media.Rx.Pkt,
		RxLost:    media.Rx.Lost,
		RxJitter:  media.Rx.Jitter,
	}
	params := dbutils.GetFields(newmedia, false)
	s1, s2 := dbutils.GenInsert(params)
	q := fmt.Sprintf("insert into Session_Medias %s values %s", s1, s2)
	_, err = db.NamedExec(q, newmedia)
	return
}

func Parse(db *sqlx.DB, ip string, r io.Reader, testConfig *AlertTest) (sess *Session, err error) {
	buf, err := ioutil.ReadAll(r)
	if err != nil {
		log.Print("Error read request body : ", err.Error())
		return
	}
	body := ioutil.NopCloser(bytes.NewBuffer(buf))
	log.Printf("Received xml from %s:\n", ip)
	log.Printf("%q\n", body)

	sess = &Session{}
	xml.Unmarshal(buf, sess)

	// insert session
	err = InsertSession(db, sess, ip)
	if err != nil {
		log.Printf("%s\n", err.Error())
		return
	}

	// insert session media
	for i, m := range sess.Medialist {
		log.Printf("Type %s\n", m.Type)
		log.Printf("Enabled %d\n", m.Enabled)
		log.Printf("Raddr %s:%d\n", m.Raddr, m.Rport)
		log.Printf("Tx %-6d %-6d %-6d\n", m.Tx.Pkt, m.Tx.Lost, m.Tx.Jitter)
		log.Printf("Rx %-6d %-6d %-6d\n", m.Rx.Pkt, m.Rx.Lost, m.Rx.Jitter)

		if testConfig != nil {
			if m.Type == "video" {
				if m.Tx.Pkt == 0 && m.Tx.Lost == 0 && m.Tx.Jitter == 0 {
					m.Rx.Pkt = 100
					m.Rx.Lost = int(testConfig.TestContentRxLoss * float64(m.Rx.Pkt))
					sess.Medialist[i].Rx.Pkt = m.Rx.Pkt
					sess.Medialist[i].Rx.Lost = m.Rx.Lost
				} else {
					m.Tx.Lost = int(testConfig.TestVideoTxLoss * float64(m.Tx.Pkt))
					sess.Medialist[i].Tx.Lost = m.Tx.Lost
					m.Rx.Lost = int(testConfig.TestVideoRxLoss * float64(m.Rx.Pkt))
					sess.Medialist[i].Rx.Lost = m.Rx.Lost
				}
			} else {
				m.Tx.Lost = int(testConfig.TestAudioTxLoss * float64(m.Tx.Pkt))
				sess.Medialist[i].Tx.Lost = m.Tx.Lost
				m.Rx.Lost = int(testConfig.TestAudioRxLoss * float64(m.Rx.Pkt))
				sess.Medialist[i].Rx.Lost = m.Rx.Lost
			}
		}
		err := InsertSessionMedia(db, m, sess.Id)
		if err != nil {
			log.Printf("%s\n", err.Error())
		}
		// calculate lost ratio
		if m.Tx.Pkt > 0 {
			sess.Medialist[i].Tx.LostRatio = float64(m.Tx.Lost / m.Tx.Pkt)
		}
		if m.Rx.Pkt > 0 {
			sess.Medialist[i].Rx.LostRatio = float64(m.Rx.Lost / m.Rx.Pkt)
		}

		if testConfig != nil {
			if m.Type == "video" {
				if m.Tx.Pkt == 0 && m.Tx.Lost == 0 && m.Tx.Jitter == 0 {
					sess.Medialist[i].Rx.LostRatio = testConfig.TestContentRxLoss
				} else {
					sess.Medialist[i].Tx.LostRatio = testConfig.TestVideoTxLoss
					sess.Medialist[i].Rx.LostRatio = testConfig.TestVideoRxLoss
				}
			} else {
				sess.Medialist[i].Tx.LostRatio = testConfig.TestAudioTxLoss
				sess.Medialist[i].Rx.LostRatio = testConfig.TestAudioRxLoss
			}
		}
	}
	return
}

func GetSessions(db *sqlx.DB) (sessions []SessionDbWithHost, err error) {
	sessions = make([]SessionDbWithHost, 0)
	q := "select s.*, r.Hostname from Sessions as s join Rend_Servers as r on s.RendServer=r.IpAddr order by s.StartUtc"
	rows, err := db.Queryx(q)
	if err != nil {
		return
	}
	for rows.Next() {
		sess := SessionDbWithHost{}
		err = rows.StructScan(&sess)
		if err == nil {
			sessions = append(sessions, sess)
		}
	}
	return
}

func GetSessionMedias(db *sqlx.DB) (sessions map[string][]Media, err error) {
	sessions = make(map[string][]Media)
	q := "select * from Session_Medias"
	rows, err := db.Queryx(q)
	if err != nil {
		return
	}
	for rows.Next() {
		media := Media{}
		err = rows.StructScan(&media)
		if err == nil {
			if _, ok := sessions[media.SessionId]; !ok {
				sessions[media.SessionId] = make([]Media, 0)
			}
			sessions[media.SessionId] = append(sessions[media.SessionId], media)
		}
	}
	return
}
