package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/milescrabill/mirror-server/config"
	"github.com/plaid/plaid-go/plaid"
	"github.com/urfave/negroni"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/securecookie"
)

var (
	conf config.Config
	pc   *plaid.Client
	s    *securecookie.SecureCookie
)

func makeSecureCookie(unencrypted map[string]string) *http.Cookie {
	encoded, err := s.Encode("plaidToken", unencrypted)
	if err != nil {
		log.Printf("[error] plaid: couldn't encode cookie %s", err)
		return nil
	}

	return &http.Cookie{
		Name:  "plaidToken",
		Value: encoded,
		// lasts a year, arbitrary
		Expires: time.Now().Add(time.Hour * 24 * 365),
	}
}

func readSecureCookie(r *http.Request, name string) map[string]string {
	if cookie, err := r.Cookie(name); err == nil {
		value := make(map[string]string)
		if err = s.Decode(name, cookie.Value, &value); err == nil {
			return value
		} else {
			log.Printf("readSecureCookie Decode error: %s", err.Error())
		}
	}
	return nil
}

func getPlaidData(token string) ([]byte, error) {
	postRes, err := pc.ExchangeToken(token)
	if err != nil {
		log.Printf("[error] plaid ExchangeToken: %s", err)
		return []byte{}, err
	}

	// Use the returned Plaid API access_token to retrieve
	// account information.
	oneWeekAgo := time.Now().AddDate(0, 0, -7).Format("01-02-2006")
	connectRes, mfaRes, err := pc.ConnectGet(postRes.AccessToken, &plaid.ConnectGetOptions{
		GTE: oneWeekAgo,
	})
	if err != nil {
		log.Printf("[error] plaid ConnectGet: %s", err.Error())
		return []byte{}, err
	}
	if mfaRes != nil {
		log.Printf("[info] plaid MFA response: %v", mfaRes)
	}

	categories := make(map[string]int)
	for _, transaction := range connectRes.Transactions {
		// for _, category := range transaction.Category {
		// 	categories[category]++
		// }

		// one category per transaction
		if len(transaction.Category) > 0 {
			categories[transaction.Category[0]]++
		}
	}

	js, err := json.Marshal(struct {
		Categories   map[string]int
		Transactions []plaid.Transaction
		Authorized   bool
	}{
		categories,
		connectRes.Transactions,
		true,
	})
	if err != nil {
		log.Printf("[error] plaid json.Marshal: fail!")
		return []byte{}, err
	}
	return js, nil
}

func PlaidHandler(w http.ResponseWriter, req *http.Request) {
	tok := struct {
		Token string
	}{}
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Printf("[error] plaid ReadAll: couldn't read request body: %s", err)
		return
	}
	if string(body) == "" {
		log.Printf("[warning] plaid: request body is empty")
		return
	}
	err = json.Unmarshal(body, &tok)
	if err != nil {
		log.Printf("[error] plaid Unmarshal: couldn't decode json request body %s: %s", body, err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	val := map[string]string{
		"plaidToken": tok.Token,
	}
	// nil cookie
	if cookie := makeSecureCookie(val); cookie == nil {
		log.Printf("[error] could not make secure cookie")
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		http.SetCookie(w, cookie)
	}

	js, err := getPlaidData(tok.Token)
	if err != nil {
		log.Printf("[error] could not get plaid data")
		w.WriteHeader(http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func BudgetHandler(w http.ResponseWriter, req *http.Request) {
	// for _, cookie := range req.Cookies() {
	// 	log.Printf("%s: %s", cookie.Name, cookie.Value)
	// }
	cookie := readSecureCookie(req, "plaidToken")
	if cookie != nil {
		if _, ok := cookie["plaidToken"]; ok {
			js, err := getPlaidData(cookie["plaidToken"])
			if err != nil {
				log.Printf("[error] could not get plaid data")
				w.WriteHeader(http.StatusInternalServerError)
			}

			w.Header().Set("Content-Type", "application/json")
			w.Write(js)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"auth": "none"}`))
}

func WeatherHandler(w http.ResponseWriter, req *http.Request) {
	log.Printf("[info] weather: got request to %q", req.RequestURI)

	resp, err := http.Get("https://api.forecast.io/forecast/" + conf.ForecastToken + "/37.8267,-122.423")
	if err != nil {
		log.Fatal(err.Error())
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err.Error())
	}
	w.Write(body)
}

func main() {

	conf = config.GetConfig()
	if conf.Port == 0 {
		conf.Port = 8000
	}
	s = securecookie.New(
		[]byte(conf.SecureCookieHashKey),
		[]byte(conf.SecureCookieBlockKey)
	)

	pc = plaid.NewClient(conf.PlaidClientID, conf.PlaidSecret, plaid.Tartan)

	r := mux.NewRouter()
	r.HandleFunc("/weather", WeatherHandler)
	r.HandleFunc("/budget", BudgetHandler)
	r.HandleFunc("/plaid_auth", PlaidHandler).Methods("POST")

	n := negroni.Classic() // Includes some default middlewares
	n.UseHandler(handlers.CORS(
		handlers.AllowCredentials(),
		handlers.AllowedHeaders([]string{"Content-Type"}),
		handlers.AllowedMethods([]string{"POST", "GET", "HEAD", "OPTIONS"}),
		handlers.AllowedOrigins([]string{"http://localhost:4000"}),
	)(r))

	n.Run(":" + strconv.Itoa(conf.Port))
}
