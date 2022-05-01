package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/PuerkitoBio/goquery"
	"go.mongodb.org/mongo-driver/bson"
	"gopkg.in/mgo.v2"
)

type DatabaseConfigurations struct {
	URL string
}

type RequestConfigurations struct {
	UserAgent string `toml:"user_agent"`
	Cookie    string
}

type Configurations struct {
	Database     DatabaseConfigurations
	Request      RequestConfigurations
	Target       string
	TimeInterval int `toml:"time_interval"`
}

type LegacyPost struct {
	PostID int
	Title  string
	Author struct {
		Instance string `json:"_instance"`
	}
	Forum struct {
		ForumID      int
		Name         string
		InternalName string
		Instance     string `json:"_instance"`
	}
	Top         int
	SubmitTime  int
	IsValid     bool
	LatestReply struct {
		Author struct {
			Instance string `json:"_instance"`
		}
		ReplyTime int
		Content   string
		Instance  string `json:"_instance"`
	}
	RepliesCount int
	Instance     string `json:"_instance"`
}

type LegacyPostList struct {
	Status int
	Data   struct {
		Count  int
		Result []LegacyPost
	}
}

type DBComment struct {
	SendTime int64
	Author   string
	AuthorId string
	Content  string
}

type DBDiscussTemplate struct {
	PostID   int
	Author   string
	AuthorID string
	SendTime int64
	Title    string
	Describe string
	Count    int
	Comment  []DBComment
}

func ChangeDiscussToDBDiscussTemlate(config Configurations, PostID int) (result DBDiscussTemplate) {
	//Complete !
	url := "https://www.luogu.com.cn/discuss/" + strconv.Itoa(PostID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("[ERROR] Tool can`t get Luogu discuss now.\n")
		time.Sleep(120 * time.Second)
		return
	}
	req.Header.Set("Cookie", config.Request.Cookie)
	req.Header.Set("Host", "www.luogu.com.cn")
	req.Header.Set("Referer", "https://www.luogu.com.cn")
	req.Header.Set("User-Agent", config.Request.UserAgent)
	client := &http.Client{Timeout: time.Second * 15}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("[ERROR] Tool can`t get Luogu discuss now.\n")
		fmt.Print("[ERROR]Error reading response. ", err)
		time.Sleep(120 * time.Second)
		return
	}
	defer resp.Body.Close()
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		fmt.Printf("[ERROR] Tool can`t get Luogu discuss now.\n")
		fmt.Print("[ERROR]Error reading response. ", err)
		time.Sleep(120 * time.Second)
		return
	}
	result.Count = 0
	titles := doc.Find("h1").First().Text()
	result.Title = titles
	result.PostID = PostID

	result.Count = 0
	//fmt.Printf(titles)
	// 获取每条评论的发布时间和内容以及整个帖子内容
	doc.Find(".am-comment-meta").Each(func(i int, selection *goquery.Selection) {
		texts := selection.Find("a").First().Text()
		if i == 0 {
			oldT := selection.Text()
			regR, _ := regexp.Compile(`[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}`)
			sendTimes, _ := time.Parse("2006-01-02 15:04", regR.FindString(oldT))
			result.SendTime = sendTimes.Unix()
			result.Author = texts
			AuthorId, _ := selection.Find("a").First().Attr("href")
			AuthorId = strings.Trim(AuthorId, "/user/")
			result.AuthorID = AuthorId
			return
		}
		result.Count++
		oldT := selection.Text()
		regR, _ := regexp.Compile(`[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}`)
		//result.Comment[i].Author = texts
		var newComment DBComment
		newComment.Author = texts
		AuthorId, _ := selection.Find("a").First().Attr("href")
		AuthorId = strings.Trim(AuthorId, "/user/")
		result.AuthorID = AuthorId
		sendTimes, _ := time.Parse("2006-01-02 15:04", regR.FindString(oldT))
		newComment.SendTime = sendTimes.Unix()
		result.Comment = append(result.Comment, newComment)
		//fmt.Println("i", i, "select text", texts)
	})
	//每条内容内容获取和标题内容
	doc.Find(".am-comment-bd").Each(func(i int, selection *goquery.Selection) {
		htmls, _ := selection.Html()
		if i == 0 {
			result.Describe = htmls
			return
		}
		result.Comment[i-1].Content = htmls
		//	fmt.Println("i", i, "select text", htmls)
	})
	// 多页面
	// TODO: 页面过多分段处理问题
	for i := 2; true; i++ {
		url = "https://www.luogu.com.cn/discuss/" + strconv.Itoa(PostID) + "?page=" + strconv.Itoa(i)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			//fmt.Printf("[ERROR] Tool can`t get Luogu discuss now.\n")
			break
		}
		req.Header.Set("Cookie", config.Request.Cookie)
		req.Header.Set("Host", "www.luogu.com.cn")
		req.Header.Set("Referer", "https://www.luogu.com.cn")
		req.Header.Set("User-Agent", config.Request.UserAgent)
		client := &http.Client{Timeout: time.Second * 15}
		resp, err := client.Do(req)
		if err != nil {
			//	fmt.Printf("[ERROR] Tool can`t get Luogu discuss now.\n")
			//	fmt.Print("[ERROR]Error reading response. ", err)
			break
		}
		defer resp.Body.Close()
		newDoc, _ := goquery.NewDocumentFromReader(resp.Body)
		nowCounts := result.Count
		newDoc.Find(".am-comment-meta").Each(func(i int, selection *goquery.Selection) {
			texts := selection.Find("a").First().Text()
			if i == 0 {
				return
			}
			result.Count++
			oldT := selection.Text()
			regR, _ := regexp.Compile(`[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}`)
			//result.Comment[i].Author = texts
			var newComment DBComment
			newComment.Author = texts
			sendTimes, _ := time.Parse("2006-01-02 15:04", regR.FindString(oldT))
			newComment.SendTime = sendTimes.Unix()
			result.Comment = append(result.Comment, newComment)
			//fmt.Println("i", i, "select text", texts)
		})
		//每条内容内容获取和标题内容
		newDoc.Find(".am-comment-bd").Each(func(i int, selection *goquery.Selection) {
			htmls, _ := selection.Html()
			if i == 0 {
				return
			}
			result.Comment[i-1+nowCounts].Content = htmls
			//	fmt.Println("i", i, "select text", htmls)
		})
		if nowCounts == result.Count { // 到头啦
			break
		}
	}
	//	fmt.Print(result)
	return
}

func SaveNewDiscuss(session *mgo.Session, config Configurations, PostID int) {
	// 检查是否已经存在
	var discuss DBDiscussTemplate
	discussCount, err := session.DB("luogulo").C("discuss").Find(bson.M{"postid": PostID}).Count()
	if err != nil {
		fmt.Print("[Save ERROR] Can`t check exist. LOG:", err)
		return
	}
	if discussCount == 0 {
		// 爬全部
		//	fmt.Printf("HERE")
		nowThings := ChangeDiscussToDBDiscussTemlate(config, PostID)
		// 分析帖子
		//nowThingsDB, err := bson.Marshal(&nowThings)
		//if err != nil {
		//	fmt.Print("[Save ERROR] ERROR. LOG:", err)
		//}
		err = session.DB("luogulo").C("discuss").Insert(&nowThings)
		if err != nil {
			fmt.Print("[Save ERROR] ERROR1. LOG:", err, "\n")
		}
	} else {
		err = session.DB("luogulo").C("discuss").Find(bson.M{"postid": PostID}).One(&discuss)
		if err != nil {
			fmt.Print("[Save ERROR] Can`t read discuss information before. LOG:", err, "\n")
			return
		}
		// 先看看爬到的最后一条的发布时间
		var lastTime int64
		if discuss.Count > 0 {
			lastTime = discuss.Comment[discuss.Count-1].SendTime
		} else {
			lastTime = 0
		}

		// 分析现在的帖子
		nowThings := ChangeDiscussToDBDiscussTemlate(config, PostID)
		// 将新评论整理
		lens := nowThings.Count
		NewDiscuss := discuss
		for i := 0; i < lens; i++ {
			if nowThings.Comment[i].SendTime > lastTime {
				NewDiscuss.Comment = append(NewDiscuss.Comment, nowThings.Comment[i])
				NewDiscuss.Count++
			} else if nowThings.Comment[i].SendTime == lastTime { // 可爱的洛谷竟然只到分钟，显然有可能遇到时间问题
				flag := false
				for j := 0; j < discuss.Count; j++ {
					if discuss.Comment[j].Content == nowThings.Comment[j].Content {
						flag = true
						break
					}
				}
				if !flag {
					NewDiscuss.Comment = append(NewDiscuss.Comment, nowThings.Comment[i])
					NewDiscuss.Count++
				}
			}
		}
		// 将内容update.
		err = session.DB("luogulo").C("discuss").Update(&discuss, &NewDiscuss)
		if err != nil {
			fmt.Print("[Save ERROR] ERROR2. LOG:", err, "\n")
		}
	}
}

func AutoSave(session *mgo.Session, config Configurations) {
	url := config.Target
	fmt.Println("Listing", url)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		panic(err)
	}
	client := http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	} else if resp.StatusCode != http.StatusOK {
		panic(resp.Status)
	}

	var result LegacyPostList
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		panic(err)
	} else if result.Status != http.StatusOK {
		panic(result.Status)
	}

	for _, post := range result.Data.Result {
		fmt.Printf("\tSaving %d...\n", post.PostID)
		SaveNewDiscuss(session, config, post.PostID)
	}

	fmt.Println("Fetched", url)
	time.Sleep(time.Duration(config.TimeInterval) * time.Second)
}

func main() {
	defer func() {
		if err := recover(); err != nil {
			fmt.Print("[ERROR] ", err, "\n")
			time.Sleep(1 * time.Second)
			fmt.Print("[INFO] Restart. \n")
			main()
		}
	}()
	var config Configurations
	if _, err := toml.DecodeFile(os.Args[1], &config); err != nil {
		panic(err)
	}
	if session, err := mgo.Dial(config.Database.URL); err != nil {
		panic(err)
	} else {
		defer session.Close()
		AutoSave(session, config)
	}
}
