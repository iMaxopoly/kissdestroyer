package main

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/anaskhan96/soup"
	"github.com/mozillazg/go-slugify"
	"github.com/robertkrimen/otto"
	"gopkg.in/go-playground/pool.v3"
	"io/ioutil"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
)

type report_structure struct {
	page_title       string
	site_url         string
	cyberlocker_urls []string
}

type myclient struct {
	fileName string
	data     []string
}

type destroy struct {
	base_url                     string
	grabber                      grab
	pool_size                    uint
	clients                      []myclient
	debug_title_links_path       string
	debug_episode_links_path     string
	debug_cyberlocker_links_path string

	get_client_mutex sync.Mutex
	file_write_mutex sync.Mutex
}

// store_clients stores clients into myclient struct from ./myclients folder
func (d *destroy) store_clients(clients_folder_path string) {
	myclientsDir, err := ioutil.ReadDir(clients_folder_path)
	if err != nil {
		if strings.Contains(err.Error(), "The system cannot find the file specified") {
			log.Println("No client files detected, progressing without clients...")
			log.Fatal(err)
			return
		}
		log.Fatal(err)
	}

	for _, f := range myclientsDir {
		if f.IsDir() {
			continue
		}
		sClient := myclient{}
		sClient.fileName = strings.TrimSuffix(f.Name(), ".txt")

		// read client files
		file, err := os.Open(clients_folder_path + "/" + f.Name())
		if err != nil {
			log.Fatal(err)
		}

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			sClient.data = append(sClient.data, strings.TrimSpace(scanner.Text()))
		}

		err = scanner.Err()
		if err != nil {
			log.Fatal(err)
		}

		err = file.Close()
		if err != nil {
			log.Fatal(err)
		}

		sort.Strings(sClient.data)

		d.clients = append(d.clients, sClient)
	}

	log.Println("Loaded clients", len(d.clients))
}

func (d *destroy) get_client(anime_title_laced_string string) (client_name string) {
	d.get_client_mutex.Lock()
	defer d.get_client_mutex.Unlock()
	for _, client := range d.clients {
		for _, title := range client.data {
			if strings.Contains(slugify.Slugify(anime_title_laced_string), slugify.Slugify(title)) {
				return client.fileName
			}
		}
	}
	return "nil"
}

func (d *destroy) process_anime_pool(title_link, client_name string) pool.WorkFunc {
	return func(wu pool.WorkUnit) (interface{}, error) {

		// pluck episodes
		episode_links, err := d.pull_episodes(title_link)
		if err != nil {
			log.Println(err)
			return nil, err
		}

		// glide through episode links
		for _, episode_link := range episode_links {
			// pluck cyberlockers
			cyberlocker_report, err := d.pull_cyberlockers(episode_link)

			// write the report
			d.file_write_mutex.Lock()
			file, err := os.OpenFile(d.debug_cyberlocker_links_path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
			if err != nil {
				log.Fatal(err)
			}
			for _, cyberlocker_link := range cyberlocker_report.cyberlocker_urls {
				_, err = file.WriteString(
					client_name +
						"<<@>>" + cyberlocker_report.page_title +
						"<<@>>" + cyberlocker_report.site_url +
						"<<@>>" + cyberlocker_link + "\n",
				)
				if err != nil {
					log.Println(err)
				}
			}
			err = file.Close()
			if err != nil {
				log.Println(err)
			}
			d.file_write_mutex.Unlock()
		}

		if wu.IsCancelled() {
			return nil, nil
		}

		return nil, nil
	}
}

func (d *destroy) Process() {
	log.Println("Commencing code: Setting pool size and starting... please wait.")
	p := pool.NewLimited(d.pool_size)
	defer p.Close()

	batch := p.Batch()

	go func() {
		// glide through titles pages
		file, err := os.Open(d.debug_title_links_path)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			client_name := d.get_client(scanner.Text())
			if client_name == "nil" {
				continue
			}
			batch.Queue(d.process_anime_pool(scanner.Text(), client_name))
		}

		if err := scanner.Err(); err != nil {
			log.Fatal(err)
		}

		batch.QueueComplete()
	}()

	for range batch.Results() {

	}
}

func (d *destroy) pull_titles_pool(page_num int) pool.WorkFunc {
	return func(wu pool.WorkUnit) (interface{}, error) {

		result_title_links, err := d.pull_titles(page_num)
		if err != nil {
			return nil, err
		}

		if wu.IsCancelled() {
			return nil, nil
		}

		return result_title_links, nil
	}
}

func (d *destroy) pull_titles(page_num int) (result_title_links []string, err error) {
	page_success := false
	proxy_pos := 0
	proxy := ""

	for !page_success {
		proxy, proxy_pos = d.grabber.get_proxy(proxy_pos)
		body_string, err := d.grabber.by_the_proxy(proxy, fmt.Sprintf("%s/AnimeList?page=%d", d.base_url, page_num))
		if err != nil {
			log.Println(err)
			proxy_pos++
			continue
		}

		if !strings.Contains(body_string, `<table class="listing">`) {
			return nil, errors.New("Table listing not found, moving on.")
		}

		doc := soup.HTMLParse(body_string)

		title_links := doc.Find("table", "class", "listing").FindAll("a")
		for _, link := range title_links {
			title_link := strings.TrimSpace(link.Attrs()["href"])
			if title_link == "" {
				continue
			}
			result_title_links = append(result_title_links, d.base_url+title_link)
		}
		page_success = true
	}

	if len(result_title_links) < 1 {
		return nil, errors.New("Empty results, moving on.")
	}

	return result_title_links, nil
}

func (d *destroy) Pull_titles() {
	log.Println("Commencing code: Pull Anime Titles... please wait.")
	p := pool.NewLimited(d.pool_size)
	defer p.Close()

	batch := p.Batch()

	go func() {
		for i := 1; i <= 142; i++ {
			batch.Queue(d.pull_titles_pool(i))
		}

		batch.QueueComplete()
	}()

	for title_links_list := range batch.Results() {
		if err := title_links_list.Error(); err != nil {
			continue
		}

		result_title_links := title_links_list.Value().([]string)
		if len(result_title_links) < 1 {
			continue
		}
		file, err := os.OpenFile(d.debug_title_links_path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			log.Fatal(err)
		}
		for _, link := range result_title_links {
			_, err = file.WriteString(link + "\n")
			if err != nil {
				log.Println(err)
			}
		}
		err = file.Close()
		if err != nil {
			log.Println(err)
		}
	}
}

func (d *destroy) pull_episodes_pool(title_link string) pool.WorkFunc {
	return func(wu pool.WorkUnit) (interface{}, error) {

		result_episode_links, err := d.pull_episodes(title_link)
		if err != nil {
			return nil, err
		}

		if wu.IsCancelled() {
			return nil, nil
		}

		return result_episode_links, nil
	}
}

func (d *destroy) pull_episodes(title_link string) (result_episode_links []string, err error) {
	page_success := false
	proxy_pos := 0
	proxy := ""

	for !page_success {
		proxy, proxy_pos = d.grabber.get_proxy(proxy_pos)
		body_string, err := d.grabber.by_the_proxy(proxy, title_link)
		if err != nil {
			log.Println(err)
			proxy_pos++
			continue
		}
		if !strings.Contains(body_string, `<table class="listing">`) {
			return nil, errors.New("Table listing not found, moving on.")
		}

		doc := soup.HTMLParse(body_string)

		episode_links := doc.Find("table", "class", "listing").FindAll("a")
		for _, link := range episode_links {
			episode_link := strings.TrimSpace(link.Attrs()["href"])
			if episode_link == "" {
				continue
			}
			result_episode_links = append(result_episode_links, d.base_url+episode_link)
		}
		page_success = true
	}

	if len(result_episode_links) < 1 {
		return nil, errors.New("Empty results, moving on.")
	}

	return result_episode_links, nil
}

func (d *destroy) Pull_episodes() {
	log.Println("Commencing code: Pull Anime Episodes... please wait.")
	p := pool.NewLimited(d.pool_size)
	defer p.Close()

	batch := p.Batch()

	// open title links
	go func() {
		file, err := os.Open(d.debug_title_links_path)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			batch.Queue(d.pull_episodes_pool(scanner.Text()))
		}

		if err := scanner.Err(); err != nil {
			log.Fatal(err)
		}

		batch.QueueComplete()
	}()

	for episode_links_list := range batch.Results() {
		if err := episode_links_list.Error(); err != nil {
			continue
		}

		result_episode_links := episode_links_list.Value().([]string)
		if len(result_episode_links) < 1 {
			continue
		}
		file, err := os.OpenFile(d.debug_episode_links_path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			log.Fatal(err)
		}
		for _, link := range result_episode_links {
			_, err = file.WriteString(link + "\n")
			if err != nil {
				log.Println(err)
			}
		}
		err = file.Close()
		if err != nil {
			log.Println(err)
		}
	}
}

func (d *destroy) pull_cyberlockers_pool(episode_link string) pool.WorkFunc {
	return func(wu pool.WorkUnit) (interface{}, error) {

		result_format, err := d.pull_cyberlockers(episode_link)
		if err != nil {
			return nil, err
		}

		if wu.IsCancelled() {
			return nil, nil
		}

		if len(result_format.cyberlocker_urls) < 1 {
			return nil, errors.New("Cyberlockers empty, moving on")
		}
		return result_format, nil
	}
}

func (d *destroy) pull_cyberlockers(episode_link string) (result_format report_structure, err error) {
	page_success := false
	proxy_pos := 0
	proxy := ""

	result_format.page_title = ""
	result_format.site_url = episode_link

	for !page_success {
		proxy, proxy_pos = d.grabber.get_proxy(proxy_pos)
		body_string, err := d.grabber.by_the_proxy(proxy, episode_link)
		if err != nil {
			log.Println(err)
			proxy_pos++
			continue
		}

		if !strings.Contains(body_string, `<select id="selectQuality">`) {
			return report_structure{}, errors.New("Table listing not found, moving on.")
		}

		doc := soup.HTMLParse(body_string)
		result_format.page_title = strings.TrimSpace(
			strings.Replace(strings.Replace(doc.Find("title").Text(), "\n", "", -1), "    ", " ", -1),
		)

		hashes := doc.Find("select", "id", "selectQuality").FindAll("option")
		for _, gHashNode := range hashes {
			vm := otto.New()
			vm.Run(`
		var asp={alphabet:"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/=",lookup:null,wrap:function(a){
		if(a.length%4)throw new Error("InvalidCharacterError: 'asp.wrap' failed: The string to be wrapd is not correctly encoded.");
		var b=asp.fromUtf8(a),c=0,d=b.length;
		for(var e="";c<d;)e+=b[c]<128?String.fromCharCode(b[c++]):b[c]>191&&b[c]<224?String.fromCharCode((31&b[c++])<<6|63&b[c++]):String.fromCharCode((15&b[c++])<<12|(63&b[c++])<<6|63&b[c++]);
		return e},fromUtf8:function(a){var c,b=-1,d=[],e=[,,,];
		if(!asp.lookup){for(c=asp.alphabet.length,asp.lookup={};++b<c;)asp.lookup[asp.alphabet.charAt(b)]=b;b=-1}
		for(c=a.length;++b<c&&(e[0]=asp.lookup[a.charAt(b)],e[1]=asp.lookup[a.charAt(++b)],d.push(e[0]<<2|e[1]>>4),
		e[2]=asp.lookup[a.charAt(++b)],64!=e[2])&&(d.push((15&e[1])<<4|e[2]>>2),e[3]=asp.lookup[a.charAt(++b)],64!=e[3]);)d.push((3&e[2])<<6|e[3]);return d}};
		abc = asp.wrap("` + strings.TrimSpace(gHashNode.Attrs()["value"]) + `")
		`)
			value, err := vm.Get("abc")
			if err != nil {
				log.Println(err)
				continue
			}

			value_string, err := value.ToString()
			if err != nil {
				log.Println(err)
				continue
			}
			google_link := strings.TrimSpace(value_string)
			if google_link == "" {
				continue
			}
			result_format.cyberlocker_urls = append(result_format.cyberlocker_urls, google_link)
		}
		page_success = true
	}

	if len(result_format.cyberlocker_urls) < 1 {
		return report_structure{}, errors.New("Empty results, moving on.")
	}

	return result_format, nil
}

func (d *destroy) Pull_cyberlockers() {
	log.Println("Commencing code: Pull Anime Cyberlockers... please wait.")
	p := pool.NewLimited(d.pool_size)
	defer p.Close()

	batch := p.Batch()

	// open episode links
	go func() {
		file, err := os.Open(d.debug_episode_links_path)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			batch.Queue(d.pull_cyberlockers_pool(scanner.Text()))
		}

		if err := scanner.Err(); err != nil {
			log.Fatal(err)
		}

		batch.QueueComplete()
	}()

	for episode_links_list := range batch.Results() {
		if err := episode_links_list.Error(); err != nil {
			continue
		}

		result_report_format := episode_links_list.Value().(report_structure)
		file, err := os.OpenFile(d.debug_cyberlocker_links_path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			log.Fatal(err)
		}
		for _, cyberlocker_link := range result_report_format.cyberlocker_urls {
			_, err = file.WriteString(
				"nil" +
					"<<@>>" + result_report_format.page_title +
					"<<@>>" + result_report_format.site_url +
					"<<@>>" + cyberlocker_link + "\n",
			)
			if err != nil {
				log.Println(err)
			}
		}
		err = file.Close()
		if err != nil {
			log.Println(err)
		}
	}
}
