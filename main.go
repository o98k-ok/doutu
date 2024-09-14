package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/netutil"
	"github.com/google/uuid"
	"github.com/nfnt/resize"
	"github.com/o98k-ok/lazy/v2/alfred"
)

var (
	CachePathKey = "cache_path"
	ResizeWidth  = "resize_width"
	MaxCount     = "max_count"
)

type ResponseList []struct {
	Path   string `json:"path"`
	Width  int    `json:"width"`
	HashID string `json:"hashId"`
	Height int    `json:"height"`
}

type ResponseListSougou struct {
	Status int    `json:"status"`
	Info   string `json:"info"`
	Data   struct {
		GroupList []any `json:"groupList"`
		TagList   []any `json:"tagList"`
		Emotions  []struct {
			ThumbSrc string `json:"thumbSrc"`
			Idx      int    `json:"idx"`
			Source   string `json:"source"`
		} `json:"emotions"`
	} `json:"data"`
}

func query(key string) ResponseList {
	dest := url.QueryEscape(key)
	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://www.dbbqb.com/api/search/json?start=0&w="+dest, nil)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Client-Id", "")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", "Hm_lvt_7d2469592a25c577fe82de8e71a5ae60=1721633603; HMACCOUNT=4873E08E7D2507F8; Hm_lpvt_7d2469592a25c577fe82de8e71a5ae60=1721633615")
	req.Header.Set("Referer", "https://www.dbbqb.com/s?w="+dest)
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36")
	req.Header.Set("Web-Agent", "web")
	req.Header.Set("sec-ch-ua", `"Not/A)Brand";v="8", "Chromium";v="126", "Google Chrome";v="126"`)
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-ch-ua-platform", `"macOS"`)
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	bodyText, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	var list ResponseList
	json.Unmarshal(bodyText, &list)
	return list
}

func queryV2(key string) ResponseListSougou {
	client := &http.Client{}
	dest := url.QueryEscape(key)
	req, err := http.NewRequest("GET", fmt.Sprintf("https://pic.sogou.com/napi/wap/emoji/searchlist?keyword=%s&spver=&rcer=&tag=0&routeName=emosearch", dest), nil)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Cookie", "SMYUV=1721632785108869; SUV=004C36F7DB8DDDCA66A07ECCDAC5F875; PIC_DEBUG=off; wuid=1721794252127; FUV=7039de3bcfc32309adc1c127c0dd9eb6; ABTEST=0|1721794465|v1")
	req.Header.Set("Referer", "https://pic.sogou.com/pic/emo/searchList.jsp?keyword=%E4%BD%A0%E5%A5%BD&spver=&rcer=&tag=1&routeName=emosearch")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36")
	req.Header.Set("sec-ch-ua", `"Not/A)Brand";v="8", "Chromium";v="126", "Google Chrome";v="126"`)
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-ch-ua-platform", `"macOS"`)
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	bodyText, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	var list ResponseListSougou
	json.Unmarshal(bodyText, &list)
	return list
}

func main() {
	envs, err := alfred.FlowVariables()
	if err != nil {
		alfred.InputErrItems("read env failed " + err.Error()).Show()
		return
	}
	path := envs[CachePathKey]
	if len(path) == 0 {
		p, _ := os.Getwd()
		path = p + "/data"
	}
	width := envs[ResizeWidth]
	if len(width) == 0 {
		width = "140"
	}
	os.MkdirAll(path, 0755)

	cli := alfred.NewApp("doutu utils")
	cli.Bind("uget", func(s []string) {
		items := alfred.NewItems()

		var urls []string
		err := json.Unmarshal([]byte(strings.Join(s, "")), &urls)
		if err != nil {
			alfred.Log("unmarshal " + strings.Join(s, "") + err.Error())
			return
		}

		count, _ := convertor.ToInt(envs[MaxCount])
		length := len(urls)
		if length > int(count) {
			length = int(count)
		}
		items.Items = make([]*alfred.Item, length)

		groups := sync.WaitGroup{}
		groups.Add(length)
		for i := 0; i < length; i++ {
			filename := path + "/" + uuid.New().String() + ".jpg"
			go func(index int, file string) {
				defer groups.Done()
				netutil.DownloadFile(file, urls[index])

				f, err := os.Open(file)
				if err != nil {
					alfred.Log("open " + err.Error())
					return
				}
				defer f.Close()

				var item *alfred.Item
				_, format, _ := image.Decode(f)
				if format == "gif" {
					_, ok := IsGifAndReturnFirstFrame(file)
					if ok {
						item = alfred.NewItem(fmt.Sprintf("GIF 表情 %d", index), "", file)
					} else {
						item = alfred.NewItem(fmt.Sprintf("表情 %d", index), "", file)
					}
				} else {
					item = alfred.NewItem(fmt.Sprintf("表情 %d", index), "", file)
				}
				item.Icon = &alfred.Icon{}
				item.WithIcon(file)
				items.Items[index] = item
			}(i, filename)
		}
		groups.Wait()
		items.Show()
	})

	cli.Bind("resize", func(s []string) {
		w, _ := convertor.ToInt(width)
		f, err := os.Open(s[0])
		if err != nil {
			alfred.Log("open " + err.Error())
			return
		}
		defer f.Close()

		normalIMG, format, err := image.Decode(f)
		if err != nil || format == "gif" {
			img, ok := IsGifAndReturnFirstFrame(s[0])
			if ok {
				return
			}
			normalIMG = img
		}

		buffer := bytes.Buffer{}
		jpeg.Encode(&buffer, ResizeImage(normalIMG, int(w)), nil)
		os.Remove(s[0])
		os.WriteFile(s[0], buffer.Bytes(), 0644)
	})

	cli.Run(os.Args)
}

func IsGifAndReturnFirstFrame(path string) (image.Image, bool) {
	frames, err := GifHandle(path)
	if err != nil {
		return nil, false
	}

	if len(frames) < 5 {
		return frames[0], false
	}
	return frames[0], true
}

func GifHandle(path string) ([]*image.Paletted, error) {
	f, err := os.Open(path)
	if err != nil {
		return []*image.Paletted{}, err
	}
	defer f.Close()

	gifs, err := gif.DecodeAll(f)
	if err != nil {
		return []*image.Paletted{}, err
	}
	return gifs.Image, nil
}

func ResizeImage(img image.Image, width int) image.Image {
	if img.Bounds().Dx() <= width {
		return img
	}
	rate := float32(width) / float32(img.Bounds().Dx())
	height := int(float32(img.Bounds().Dy()) * rate)
	imgRes := resize.Resize(uint(width), uint(height), img, resize.Lanczos3)
	return imgRes
}
