package main

import (
	"fmt"
	"os"
	"strings"

	"database/sql"
	"log"
	"net/smtp"

	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB

type SKIN struct{
    ID int64
    NAME string
    GUN_NAME string
}

type NOTIF_DATA struct{
    SKIN_NAME string
    GUN_NAME string
    EMAIL string
}

type Secret struct{
    email string
    password string
}

func getSecrets() Secret{
  file,err1 := os.Open(".secrets")
  m := make(map[string]string)
  if err1 != nil{
      log.Fatal(err1)
  }
  data := make([]byte,100);
  count,err2 := file.Read(data)
  if err2 != nil{
      log.Fatal(err2)
  }

  secrets := strings.Split(strings.TrimSpace(string(data[:count])),"\n");

  for  _,s := range secrets{
      tmp := strings.Split(s,"=")
      m[strings.TrimSpace(tmp[0])] = strings.TrimSpace(tmp[1])
  }

  return Secret{m["EMAIL"],m["PASSWORD"]}
}

func tmp(){
    db,err := sql.Open("mysql","admin:admin@tcp(localhost:3306)/CS")
    res,err := db.Query(`
    SELECT s.NAME,s.GUN_NAME,u.email
    FROM 
        WATCHLIST as w,
        SKINS as s,
        User as u
    WHERE
        w.SKIN_ID = s.ID AND
        w.USER_ID = u.id;`);
    for res.Next(){
        var notifData NOTIF_DATA
        err = res.Scan(&notifData.SKIN_NAME,&notifData.GUN_NAME,&notifData.EMAIL)

    }
    defer db.Close()
    errCheck(err)
}

func sendEmail(email string, password string, message []byte,to []string){


  // smtp server configuration.
  smtpHost := "smtp.gmail.com"
  smtpPort := "587"

  // Message.
  
  // Authentication.
  auth := smtp.PlainAuth("", email, password, smtpHost)
  
  // Sending email.
  err := smtp.SendMail(smtpHost+":"+smtpPort, auth, email, to, message)
  if err != nil {
    fmt.Println(err)
    return
  }
  fmt.Println("Email Sent Successfully!")
}

func main() { 
  to := []string{
    "naviivan321@gmail.com",
  }

}

func errCheck(err error){
  if err != nil{
    log.Fatal(err)
  }
}
