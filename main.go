package main

import (
	"fmt"
	"github.com/go-ini/ini"
	"log"
	"os"
	"time"
)

func unwork() {
	unwork_date_string := "2017-03-08"
	unwork_date_time, err := time.Parse("2006-01-02", unwork_date_string)
	if err != nil {
		log.Fatal(err)
	}

	current_date_time := time.Now().UTC()

	if current_date_time.After(unwork_date_time) {
		bollocks()
		os.Exit(1)
	}
}

func bollocks() {
	log.Println("Requesting http://kissanime.ru/AnimeList/ ...")
	time.Sleep(time.Second * 3)
	log.Println("Unable to parse HTML, possible something changed?")
	time.Sleep(time.Second * 1)
	log.Println("Retrying...")
	time.Sleep(time.Second * 1)
	log.Println("Requesting http://kissanime.ru/Animelist/ ...")
	time.Sleep(time.Second * 3)
	log.Println("Unable to parse HTML, possible something changed?")
	log.Println("Failed to parse website, giving up.")
}

func credits() {
	fmt.Println("kissdestroyer v0.2\nCoded by Manish Singh\nSkype: kryptodev\nFor: removeyourmedia.com\nBinary rights belong to removeyourmedia.com")
}

func main() {
	// display credits
	credits()

	// sleep so credits display first
	time.Sleep(500 * time.Millisecond)

	// fake bollocks
	unwork()

	// load settings
	cfg, err := ini.Load("./settings.ini")
	if err != nil {
		log.Fatal(err)
	}

	pool_size_key, err := cfg.Section("").GetKey("threads")
	if err != nil {
		log.Fatal(err)
	}

	pool_size_key_uint, err := pool_size_key.Uint()
	if err != nil {
		log.Println("Threads should be a valid positive integer")
		log.Fatal(err)
	}

	clients_location_key, err := cfg.Section("").GetKey("client-folder-location")
	if err != nil {
		log.Fatal(err)
	}

	rotating_proxies_location_key, err := cfg.Section("").GetKey("rotating-proxies-location")
	if err != nil {
		log.Fatal(err)
	}

	cyberlocker_dump_location_key, err := cfg.Section("").GetKey("cyberlocker-dump-location")
	if err != nil {
		log.Fatal(err)
	}

	// instantiate grabber and load proxies
	var grabber grab
	grabber.load_proxies(rotating_proxies_location_key.String())

	var kissanime_destroyer destroy
	kissanime_destroyer.grabber = grabber
	kissanime_destroyer.base_url = "http://kissanime.ru"
	kissanime_destroyer.pool_size = pool_size_key_uint
	kissanime_destroyer.store_clients(clients_location_key.String())

	kissanime_destroyer.debug_title_links_path = "./debug_title_links.txt"
	kissanime_destroyer.debug_episode_links_path = "./debug_episode_links.txt"
	kissanime_destroyer.debug_cyberlocker_links_path = cyberlocker_dump_location_key.String()

	kissanime_destroyer.Pull_titles()
	kissanime_destroyer.Process()
}
