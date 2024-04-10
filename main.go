package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"

	"database/sql"
	"log"
	"net/http"
	"net/smtp"
	"net/url"

	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB
var watchListChan chan NOTIF_DATA;

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
    LAST_NOTIF sql.NullTime
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
    dbString string
}


type Steam struct{
    LowestPrice string `json:"lowest_price"`
    Success  bool `json:"success"`
}

var logFile *os.File
func getSecrets() Secret{
  file,err1 := os.Open("/home/tmp/.secrets")
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

  return Secret{m["EMAIL"],m["PASSWORD"],m["DB_URL"]}
}

func convertToFrontEndForm(aString string) string{
    return strings.Replace(aString,"_"," ",-1);
}

func writeToLogFile(content string){
    _,err := logFile.Write([]byte(content+"\n"))
    if err != nil {
        fmt.Println("Error when writing:\n",content,"\n to the log file\n","log writing error:",err.Error())
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
    var tmp int
    var err error
    if left != ""{
        tmp,err = strconv.Atoi(left)
    }
    if left != "" && err != nil{
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



type BitskinDbEntry struct{
    id int
    name string
    lowestPrice int
}

// the first 3 digits of any bitskins item are fractional parts of the decimal
func formatBitskinPrice(price string) string{
    for len(price) < 3{
        price += "0"+price
    }
    var right string
    var left string
    for i := len(price) -1; i >= 0;i--{
        if (len(right) < 3){
            right += string(price[i])
        }else{
            left += string(price[i])
        }
    }
    return left+"."+right
}

func bitskinsQuery(notifData NOTIF_DATA){
    res,err := db.Query("CALL GET_BITSKIN(?,?,?)",notifData.SKIN_NAME,notifData.GUN_NAME,notifData.TIER);
    if err != nil{
        writeToLogFile("Failed to fetch guns on sale from bitskins. Error: "+err.Error())
    }
    for res.Next(){
        var entry BitskinDbEntry
        err = res.Scan(&entry.id,&entry.name,&entry.lowestPrice)
        if err != nil{
            writeToLogFile("Failed to parse bitskins query result into go struct. Error: "+err.Error())
        }
        curBitskinPrice := getPrice(formatBitskinPrice(fmt.Sprint(entry.lowestPrice)))
        if isGE(getPrice(notifData.PRICE),curBitskinPrice){
            msg := "Subject: Bitskins watchlist notification \r\n\r\n" + entry.name + " is now for sale on bitskins for "+ formatBitskinPrice(fmt.Sprint(entry.lowestPrice))+ " USD.\nThis was on your watchlist for "+notifData.PRICE+" USD.";
            setNotifdataDate(notifData)
            sendEmail(notifData.EMAIL,getSecrets().password,[]byte(msg),[]string{notifData.EMAIL})
        }
    }

}

type BitskinsJsonList struct{
    Entry []BitskinsJsonEntry `json:"list"`
}

type BitskinsJsonEntry struct{
    Name string `json:"name"`
    PriceMin int `json:"price_min"`
    SkinId int `json:"skin_id"`
}


func pollBitskins(){
    url := "https://api.bitskins.com/market/insell/730"
    resp,err := http.Get(url)
    if err != nil{
        writeToLogFile("Failed to make http request to bitskins api with error: " + err.Error())
    }
    data,err := io.ReadAll(resp.Body);
    if err != nil{
        writeToLogFile("Failed to translate bitskins data to golang json object "+err.Error())
    }
    var res BitskinsJsonList
    err1 := json.Unmarshal(data,&res)
    if err1 != nil{
        writeToLogFile("Failed to unmarshal bitkskins json object: "+err1.Error())
    }
    for i := 0; i < len(res.Entry); i++{
        e := res.Entry[i]
        row,err2 := db.Query(`CALL UPDATE_BITSKIN(?,?,?);`,e.SkinId,e.Name,e.PriceMin)
        if err2 != nil{
            writeToLogFile("Failed to update bitskin entry with error: "+err2.Error())
        }
        row.Close()
    }
}

// set the notif date to now
func setNotifdataDate(notifData NOTIF_DATA){
    res,err := db.Query("CALL UPDATE_NOTIF(?,?,?,?)",notifData.SKIN_NAME,notifData.GUN_NAME,notifData.EMAIL,time.Now().UTC())
    if err != nil{
        writeToLogFile("Failed to update watchlist row last notif date with error: \n"+err.Error())
    }
    res.Close()
}

func sendSteamEmail(notifData NOTIF_DATA,steamPrice string){
    if (isGE(getPrice(notifData.PRICE),getPrice(steamPrice))){
        msg :="Subject: Steam watchlist notification \r\n\r\n" + convertToFrontEndForm(notifData.GUN_NAME) + " " +convertToFrontEndForm(notifData.SKIN_NAME) + " (" + notifData.TIER + 
        ") is now for sale for "+steamPrice + " in steam.\nThis was on your watchlist for "+notifData.PRICE +" USD";
        sendEmail(notifData.EMAIL, getSecrets().password, []byte(msg),[]string{notifData.EMAIL})
    }
}

func steamQuery(notifData NOTIF_DATA){
    url := "https://steamcommunity.com/market/priceoverview/?country=CA&currency=1&appid=730&market_hash_name=" + url.PathEscape(convertToFrontEndForm(notifData.GUN_NAME) + " | " + convertToFrontEndForm(notifData.SKIN_NAME) + " (" + notifData.TIER +")")
    resp,err := http.Get(url)
    if err != nil{
        writeToLogFile("Could not make a request to steam api for user: "+notifData.EMAIL+ "and for the gun: "+notifData.GUN_NAME+ " "+notifData.SKIN_NAME)
    }
    body, err := io.ReadAll(resp.Body)
    if strings.ToLower(resp.Status) != "200 ok" {
        writeToLogFile("Steam api NON-OK status with status"+ resp.Status + "the following url: " + url)
    }else{
        var steamPrice Steam
        err := json.Unmarshal(body,&steamPrice)
        if err != nil{
            writeToLogFile("Could not convert json result to go struct\n err: "+err.Error())
        }else{
            if (steamPrice.Success){
                setNotifdataDate(notifData)
                sendSteamEmail(notifData,steamPrice.LowestPrice)
            }
        }
    }
}

func marketQuery(){
    for{
        notifData := <- watchListChan
        go steamQuery(notifData)
        go bitskinsQuery(notifData)
        time.Sleep(1*time.Second)
    }
}


func pollWatchlist(){
    for{
        res,err := db.Query(`CALL GET_WATCHLIST()`);
        if err != nil{
            writeToLogFile("Unable to get wat")
        }
        for res.Next(){
            var notifData NOTIF_DATA
            var notifDataNoPrice NOTIF_DATA_NO_PRICE
            hasPrice := true
            err = res.Scan(&notifData.SKIN_NAME,&notifData.GUN_NAME,&notifData.EMAIL,&notifData.PRICE,&notifData.TIER,&notifData.LAST_NOTIF)
            if err != nil{
                hasPrice = false
                err = res.Scan(&notifDataNoPrice.SKIN_NAME,&notifDataNoPrice.GUN_NAME,&notifDataNoPrice.EMAIL,&notifDataNoPrice.PRICE,&notifDataNoPrice.TIER,&notifData.LAST_NOTIF)
                if err != nil{
                    writeToLogFile("Could not process user data from database")
                }
            }
            if (hasPrice){
                watchListChan <- notifData
            }
        }
        res.Close()
        if err != nil{
            writeToLogFile("Query to get watchlist failed with error " + err.Error())
        }
        time.Sleep(1*time.Second)
    }
}

func sendEmail(email string, password string, message []byte,to []string){
  smtpHost := "smtp.gmail.com"
  smtpPort := "587"
  auth := smtp.PlainAuth("", email, password, smtpHost)
  
  err := smtp.SendMail(smtpHost+":"+smtpPort, auth, email, to, message)
  if err != nil {
    writeToLogFile("could not send email with Error: "+err.Error())
    return
  }
}

func main() {
    logFileTmp,err := os.OpenFile("cs2Log.txt", os.O_WRONLY | os.O_APPEND | os.O_CREATE,0644)
    logFile = logFileTmp
    watchListChan = make(chan NOTIF_DATA)
    if err != nil{
        fmt.Printf("Could create or open log file\n");
    }
    db,_ = sql.Open("mysql",getSecrets().dbString)
    if err != nil{
        writeToLogFile("Could not open database connection to update bitskins table")
    }
    go pollWatchlist() // producer
    go marketQuery() // consumer
    for {
        //infinite loop to keep the routines running
    }
}

