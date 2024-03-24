package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode"

	"database/sql"
	"log"
	"net/http"
	"net/smtp"
	"net/url"

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
    PRICE string
    TIER string
}

type NOTIF_DATA_NO_PRICE struct{
    SKIN_NAME string
    GUN_NAME string
    EMAIL string
    PRICE *string
    TIER string
}

type Secret struct{
    email string
    password string
}


type Steam struct{
    LowestPrice string `json:"lowest_price"`
    Success  bool `json:"success"`
}

var logFile *os.File
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

func convertToFrontEndForm(aString string) string{
    return strings.Replace(aString,"_"," ",-1);
}

func writeToLogFile(content string){
    n,err := logFile.Write([]byte(content))
    if err != nil || n != len([]byte(content)){
        fmt.Println("Error when writing:\n",content,"\n to the log file")
    }
}

type DecimalDig struct{
    left int
    right string
}

// return the (digits to the left of the dot, digits to the right of the dot)
func getPrice(price string) DecimalDig{
    isBeforeDot := true
    left := ""
    right := ""
    for _,c := range price{
        if (unicode.IsDigit(c)){
            if (isBeforeDot){
                left += string(c)
            }else{
                right += string(c)
            }
        }else if (c == '.'){
            isBeforeDot = false
        }
    }
    tmp,err := strconv.Atoi(left)
    if err != nil{
        writeToLogFile("could not convert error: " + err.Error() +"\n")
    }
    return DecimalDig{left:tmp,right:right}
}

func isGEFractional(frac1 string, frac2 string) bool{
    for idx := 0 ;idx < len(frac1) || idx < len(frac2);idx++{
        if (idx < len(frac1) && idx < len(frac2)){
            if (int(frac1[idx]) > int(frac2[idx])){
                return true
            }else if (int(frac1[idx]) < int(frac2[idx])){
                return false
            }
        }else if (idx < len(frac1)){
            return true
        }else if (idx < len(frac2)){
            return false
        }
    }
    return true // means they are equal
}

func isGE(dig1 DecimalDig,dig2 DecimalDig) bool{
    if (dig1.left > dig2.left){
        return true;
    }else if (dig1.left == dig2.left && isGEFractional(dig1.right,dig2.right)){
        return true;
    }
    return false;
}



func steamQuery(notifData NOTIF_DATA){
    url := "https://steamcommunity.com/market/priceoverview/?country=CA&currency=1&appid=730&market_hash_name=" + url.PathEscape(convertToFrontEndForm(notifData.GUN_NAME) + " | " + convertToFrontEndForm(notifData.SKIN_NAME) + " (" + notifData.TIER +")")
    resp,err := http.Get(url)
    if err != nil{
        logFile.Write([]byte("Could not make a request to steam api for user: "+notifData.EMAIL+ "and for the gun: "+notifData.GUN_NAME+ " "+notifData.SKIN_NAME))
    }
    body, err := io.ReadAll(resp.Body)
    if strings.ToLower(resp.Status) != "200 ok" {
        writeToLogFile("Steam api NON-OK status with the following url: " + url)
    }else{
        var steamPrice Steam
        err := json.Unmarshal(body,&steamPrice)
        if err != nil{
            writeToLogFile("Could not convert json result to go struct\n err: "+err.Error())
        }else{
            if (steamPrice.Success){
               if (isGE(getPrice(notifData.PRICE),getPrice(steamPrice.LowestPrice))){
                msg := "Gun "+ notifData.GUN_NAME + " " + notifData.SKIN_NAME + " " + notifData.TIER + 
                " is now for sale for under "+notifData.PRICE + "USD in steam";
                sendEmail(notifData.EMAIL, getSecrets().password, []byte(msg),[]string{"naviivan321@gmail.com"})
               }
            }
        }
    }
}

func tmp(){
    db,err := sql.Open("mysql","admin:admin@tcp(localhost:3306)/CS")
    res,err := db.Query(`
    SELECT s.NAME,s.GUN_NAME,u.email,w.PRICE,w.TIER
    FROM 
        WATCHLIST as w,
        SKINS as s,
        User as u
    WHERE
        w.SKIN_ID = s.ID AND
        w.USER_ID = u.id;`);
    for res.Next(){
        var notifData NOTIF_DATA
        var notifDataNoPrice NOTIF_DATA_NO_PRICE
        hasPrice := true
        err = res.Scan(&notifData.SKIN_NAME,&notifData.GUN_NAME,&notifData.EMAIL,&notifData.PRICE,&notifData.TIER)
        if err != nil{
            hasPrice = false
            err = res.Scan(&notifDataNoPrice.SKIN_NAME,&notifDataNoPrice.GUN_NAME,&notifDataNoPrice.EMAIL,&notifDataNoPrice.PRICE,&notifDataNoPrice.TIER)
            if err != nil{
                writeToLogFile("Could not process user data from database")
            }
        }
        // query each market place
        if (hasPrice){
            steamQuery(notifData)
        }
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
    logFileTmp,err := os.OpenFile("cs2Log.txt", os.O_WRONLY | os.O_CREATE,0644)
    logFile = logFileTmp
    if err != nil{
        fmt.Printf("Could create or open log file\n");
    }
    tmp()
}

func errCheck(err error){
  if err != nil{
    log.Fatal(err)
  }
}
