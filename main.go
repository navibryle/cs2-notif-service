package main

import "fmt"
import (
    _ "github.com/go-sql-driver/mysql"
    "database/sql"
    "log"
)

var db *sql.DB

type SKIN struct{
    ID int64
    NAME string
    GUN_NAME string
}

func main() {
    db,err := sql.Open("mysql","admin:admin@tcp(localhost:3306)/CS")
    res,err := db.Query("SELECT * FROM SKINS;");
    for res.Next(){
        var skin SKIN
        err = res.Scan(&skin.ID,&skin.NAME,&skin.GUN_NAME)
        fmt.Printf("id: %d, skin name: %s, gun name: %s\n",skin.ID,skin.NAME,skin.GUN_NAME);

    }
    defer db.Close()
    errCheck(err)
}
func errCheck(err error){
  if err != nil{
    log.Fatal(err)
  }
}
