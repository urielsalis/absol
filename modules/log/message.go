package log

import (
	"github.com/bwmarrin/discordgo"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
	"github.com/lordralex/absol/api"
	"github.com/lordralex/absol/api/database"
	"github.com/lordralex/absol/api/logger"
	"github.com/spf13/viper"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
)

var lastAuditIds = make(map[string]string)
var auditLastCheck sync.Mutex
var loggedServers []string
var client = &http.Client{}

type Module struct {
	api.Module
}

func (*Module) Load(session *discordgo.Session) {
	loggedServers = strings.Split(viper.GetString("LOGGED_SERVERS"), ";")

	session.AddHandler(OnMessageCreate)
	session.AddHandler(OnMessageDelete)
	session.AddHandler(OnMessageDeleteBulk)
	session.AddHandler(OnMessageEdit)
	session.AddHandlerOnce(OnConnect)

	api.RegisterIntentNeed(discordgo.IntentsGuildMessages, discordgo.IntentsGuildBans, discordgo.IntentsGuildMembers)
}

func OnConnect(ds *discordgo.Session, mc *discordgo.Connect) {
	auditLastCheck.Lock()
	defer auditLastCheck.Unlock()

	for _, guild := range ds.State.Guilds {
		auditLog, err := ds.GuildAuditLog(guild.ID, "", "", int(discordgo.AuditLogActionMessageDelete), 1)
		if err != nil {
			logger.Err().Printf("Failed to check audit log: %s", err.Error())
		} else {
			for _, v := range auditLog.AuditLogEntries {
				lastAuditIds[guild.ID] = v.ID
			}
		}
	}
}

func downloadAttachment(db *gorm.DB, id, url, filename string) {
	//check to see if URL already exists, if so, skip
	stmt, _ := db.DB().Prepare("SELECT id from attachments WHERE url = ?")
	rows, err := stmt.Query(url)
	_ = stmt.Close()
	if err != nil {
		logger.Err().Print(err.Error())
		return
	}
	hasRows := rows.Next()
	_ = rows.Close()
	if hasRows {
		return
	}

	var data []byte
	response, err := client.Get(url)
	if err == nil {
		defer response.Body.Close()
		data, _ = ioutil.ReadAll(response.Body)
	}

	stmt, _ = db.DB().Prepare("INSERT INTO attachments (message_id, url, name, contents) VALUES (?, ?, ?, ?);")
	err = database.Execute(stmt, id, url, filename, data)
	if err != nil {
		logger.Err().Print(err.Error())
	}
}