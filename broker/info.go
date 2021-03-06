package broker

import (
	"fmt"
	"time"

	"github.com/eclipse/paho.mqtt.golang/packets"

	simplejson "github.com/bitly/go-simplejson"
	log "github.com/cihub/seelog"
)

func (c *client) SendInfo() {
	if c.status == Disconnected {
		return
	}
	url := c.info.localIP + ":" + c.broker.config.Cluster.Port

	infoMsg := NewInfo(c.broker.id, url, false)
	err := c.WriterPacket(infoMsg)
	if err != nil {
		log.Error("send info message error, ", err)
		return
	}
}

func (c *client) StartPing() {
	timeTicker := time.NewTicker(time.Second * 50)
	ping := packets.NewControlPacket(packets.Pingreq).(*packets.PingreqPacket)
	for {
		select {
		case <-timeTicker.C:
			err := c.WriterPacket(ping)
			if err != nil {
				log.Error("ping error: ", err)
				c.Close()
			}
		case _, ok := <-c.closed:
			if !ok {
				return
			}
		}
	}
}

func (c *client) SendConnect() {

	if c.status != Connected {
		return
	}
	m := packets.NewControlPacket(packets.Connect).(*packets.ConnectPacket)

	m.CleanSession = true
	m.ClientIdentifier = c.info.clientID
	m.Keepalive = uint16(60)
	err := c.WriterPacket(m)
	if err != nil {
		log.Error("send connect message error, ", err)
		return
	}
	log.Info("send connect success")
}

func NewInfo(sid, url string, isforword bool) *packets.PublishPacket {
	pub := packets.NewControlPacket(packets.Publish).(*packets.PublishPacket)
	pub.Qos = 0
	pub.TopicName = BrokerInfoTopic
	pub.Retain = false
	info := fmt.Sprintf(`{"brokerID":"%s","brokerUrl":"%s"}`, sid, url)
	// log.Info("new info", string(info))
	pub.Payload = []byte(info)
	return pub
}

func (c *client) ProcessInfo(packet *packets.PublishPacket) {
	nc := c.conn
	b := c.broker
	if nc == nil {
		return
	}

	log.Info("recv remoteInfo: ", string(packet.Payload))

	js, e := simplejson.NewJson(packet.Payload)
	if e != nil {
		log.Warn("parse info message err", e)
		return
	}

	routes, err := js.Get("data").Map()
	if routes == nil {
		log.Error("receive info message error, ", err)
		return
	}

	b.nodes = routes

	b.mu.Lock()
	for rid, rurl := range routes {
		if rid == b.id {
			continue
		}

		url, ok := rurl.(string)
		if ok {
			exist := b.CheckRemoteExist(rid, url)
			if !exist {
				b.connectRouter(rid, url)
			}
		}

	}
	b.mu.Unlock()
}
