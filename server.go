package main

import(
  //"fmt"
	"log"
  "os"
	"net/http"
  //"strconv"
  "github.com/gorilla/mux"
  "encoding/json"
  "github.com/rs/cors"
  "database/sql"
  _ "github.com/go-sql-driver/mysql"
  "flag"
)

type Config struct{
  ApiPort string `json:"api_port"`
  GoogleRecaptchaKey string `json:"google_recaptcha_key"`
  DBHost string `json:"db_host"`
  DBName string `json:"db_name"`
  DBUser string `json:"db_user"`
  DBPass string `json:"db_pass"`
}

type Guest struct {
  Name string `json:"name"`
  Address string `json:"address"`
  Email string `json:"email"`
	Attending string `json:"attending"`
	GuestOf string `json:"guestof"`
}

type Req struct {
  Email string `json:"email"`
  Key string `json:"key"`
}

type AddReq struct {
  Email string `json:"email"`
  Key string `json:"key"`
  Recaptcha string `json:"recaptcha"`
  Guests []Guest `json:"guests"`
}

type RecaptchaResp struct {
  Success bool `json:"success"`
}

type ListResp struct {
  Guests []Guest `json:"guests"`
  Success bool `json:"success"`
  Error string `json:"error"`
}

type ListLengthResp struct {
  Length uint `json:"length"`
  Success bool `json:"success"`
  Error string `json:"error"`
}

type Resp struct {
  Success bool `json:"success"`
  Error string `json:"error"`
}

var db *sql.DB
var recaptchaKey string

func RootHandler(w http.ResponseWriter, r *http.Request) {
  w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
  w.WriteHeader(http.StatusOK)
	w.Write([]byte("Hi there :)\n"))
}

func verifyRecaptcha(s string, r string) bool{
  url := "https://www.google.com/recaptcha/api/siteverify"
  resp, err := http.Get(url + "?secret=" + s + "&response=" + r)
  if err != nil {
    resp.Body.Close()
    return false
  }

  decoder := json.NewDecoder(resp.Body)
  var t RecaptchaResp
  err = decoder.Decode(&t)
  if err != nil {
    resp.Body.Close()
    return false
  }

  if(!t.Success){
    resp.Body.Close()
    return false
  }

  return true
}

func verifyApiKey(e string, a string) bool{
  if(e != "" && a != ""){
    var dbKey string
    rows, err := db.Query("SELECT ApiKey FROM users WHERE Email=?", e)
    if err != nil {
      log.Fatal(err)
    }
    defer rows.Close()
    for rows.Next() {
      err := rows.Scan(&dbKey)
      if err != nil {
        log.Fatal(err)
      }
      if(a == dbKey){
        rows.Close()
        return true
      }
    }
    rows.Close()
  }
  return false
}

func addGuests(w http.ResponseWriter, r *http.Request) {
  w.Header().Set("Content-Type", "application/json; charset=UTF-8")
  w.WriteHeader(http.StatusOK)

  defer r.Body.Close()

  j := AddReq{}
  json.NewDecoder(r.Body).Decode(&j)

  resp := Resp{}

  if(verifyRecaptcha(recaptchaKey, j.Recaptcha) || verifyApiKey(j.Email, j.Key)){
    stmt, err := db.Prepare("INSERT INTO guests (Name, Address, Email, Attending, GuestOf) VALUES (?, ?, ?, ?, ?)")
    if err != nil {
    	log.Fatal(err)
    }

    for _, g := range j.Guests {
      _, err := stmt.Exec(g.Name, g.Address, g.Email, g.Attending, g.GuestOf)
      if err != nil {
      	log.Fatal(err)
      }
    }
    resp = Resp{true, ""}
  } else {
    resp = Resp{false, "Invalid Verification"}
  }

  json.NewEncoder(w).Encode(resp)
}

func listGuests(w http.ResponseWriter, r *http.Request) {
  w.Header().Set("Content-Type", "application/json; charset=UTF-8")
  w.WriteHeader(http.StatusOK)

  defer r.Body.Close()

  j := Req{}
  json.NewDecoder(r.Body).Decode(&j)

  resp := ListResp{}

  if(verifyApiKey(j.Email, j.Key)){
    var g Guest
    rows, err := db.Query("SELECT Name, Address, Email, Attending, GuestOf FROM guests")
    if err != nil {
      log.Fatal(err)
    }
    defer rows.Close()
    for rows.Next() {
      err := rows.Scan(&g.Name, &g.Address, &g.Email, &g.Attending, &g.GuestOf)
      if err != nil {
        log.Fatal(err)
      }
      resp.Guests = append(resp.Guests, g)
    }
    rows.Close()
    resp.Success = true
  } else {
    resp.Success = false
    resp.Error = "Invalid Verification"
  }

  json.NewEncoder(w).Encode(resp)
}

func listLengthGuests(w http.ResponseWriter, r *http.Request) {
  w.Header().Set("Content-Type", "application/json; charset=UTF-8")
  w.WriteHeader(http.StatusOK)

  defer r.Body.Close()

  j := Req{}
  json.NewDecoder(r.Body).Decode(&j)

  resp := ListLengthResp{}

  if(verifyApiKey(j.Email, j.Key)){
    var g uint
    rows, err := db.Query("SELECT COUNT(id) FROM guests;")
    if err != nil {
      log.Fatal(err)
    }
    defer rows.Close()
    for rows.Next() {
      err := rows.Scan(&g)
      if err != nil {
        log.Fatal(err)
      }
      resp.Length = g
    }
    rows.Close()
    resp.Success = true
  } else {
    resp.Success = false
    resp.Error = "Invalid Verification"
  }

  json.NewEncoder(w).Encode(resp)
}

func main() {
  config_ptr := flag.String("config", "config.json", "Path to config file.")
  log_ptr := flag.Bool("log", false, "Enable logging.")
  log_path_ptr := flag.String("log_path", "mrandmrsblanton_api.log", "Path to log file.")

  flag.Parse()

  if(*log_ptr){
    f, err := os.OpenFile(*log_path_ptr, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        log.Fatal(err)
    }
    defer f.Close()
    log.SetOutput(f)
  }

  log.Println("Starting API Server...")
  log.Println("Getting config file info...")

  file, err := os.Open(*config_ptr)
  if err != nil {
    log.Println("Did you rename config.json.example to config.json? :)")
  	log.Fatal(err)
  }

  config := Config{}
  json.NewDecoder(file).Decode(&config)
  file.Close()

  recaptchaKey = config.GoogleRecaptchaKey

  log.Println("Connecting to Database...")
  //"user:pass@tcp(host:3306)/dbname"
  conn := config.DBUser + ":" + config.DBPass + "@tcp(" + config.DBHost + ")/" + config.DBName
  db, err = sql.Open("mysql", conn)
  if err != nil {
    log.Fatalf("Error initializing database connection: %s", err.Error())
  }

  err = db.Ping()
  if err != nil {
    log.Fatalf("Error opening database connection: %s", err.Error())
  }
  log.Println("Database Connected.")

  //Routes
  router := mux.NewRouter().StrictSlash(true)
  router.HandleFunc("/", RootHandler)
  router.HandleFunc("/add", addGuests)
  router.HandleFunc("/list/all", listGuests)
  router.HandleFunc("/list/length", listLengthGuests)

  //MAY NEED TO ADJUST CORS SETTINGS, DOCS HERE: https://github.com/rs/cors
  handler := cors.Default().Handler(router)
  log.Println("API Server Started.")
  if(config.ApiPort == ""){
    config.ApiPort = "8181"
  }
  err = http.ListenAndServe(":" + config.ApiPort, handler)
  log.Fatal(err)
}
