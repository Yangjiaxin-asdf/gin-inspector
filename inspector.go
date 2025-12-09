package inspector

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/url"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type Pagination struct {
	Total       int           `json:"total"`
	TotalPage   int           `json:"total_page"`
	CurrentPage int           `json:"current_page"`
	PerPage     int           `json:"per_page"`
	HasNext     bool          `json:"has_next"`
	HasPrev     bool          `json:"has_prev"`
	NextPageUrl string        `json:"next_page_url"`
	PrevPageUrl string        `json:"prev_page_url"`
	Data        []RequestStat `json:"data"`
}

type RequestStat struct {
	RequestedAt   time.Time `json:"requested_at"`
	RequestUrl    string    `json:"request_url"`
	HttpMethod    string    `json:"http_method"`
	HttpStatus    int       `json:"http_status"`
	ContentType   string    `json:"content_type"`
	GetParams     any       `json:"get_params"`
	PostParams    any       `json:"post_params"`
	Json          any       `json:"json"`
	PostMultipart any       `json:"post_multipart"`
	ClientIP      string    `json:"client_ip"`
	Cookies       any       `json:"cookies"`
	Headers       any       `json:"headers"`
}

type AllRequests struct {
	Requets []RequestStat `json:"requests"`
}

var allRequests = AllRequests{}
var pagination = Pagination{}

func GetPaginator() Pagination {
	return pagination
}

func InspectorStats() gin.HandlerFunc {
	return func(c *gin.Context) {
		urlPath := c.Request.URL.Path

		if urlPath == "/_inspector" {
			page, _ := strconv.ParseFloat(c.DefaultQuery("page", "1"), 64)
			perPage, _ := strconv.ParseFloat(c.DefaultQuery("per_page", "20"), 64)
			total := float64(len(allRequests.Requets))
			totalPage := math.Ceil(total / perPage)
			offset := (page - 1) * perPage

			if offset < 0 {
				offset = 0
			}

			pagination.HasPrev = false
			pagination.HasNext = false
			pagination.CurrentPage = int(page)
			pagination.PerPage = int(perPage)
			pagination.TotalPage = int(totalPage)
			pagination.Total = int(total)
			pagination.Data = paginate(allRequests.Requets, int(offset), int(perPage))

			if pagination.CurrentPage > 1 {
				pagination.HasPrev = true
				pagination.PrevPageUrl = urlPath + "?page=" + strconv.Itoa(pagination.CurrentPage-1) + "&per_page=" + strconv.Itoa(pagination.PerPage)
			}

			if pagination.CurrentPage < pagination.TotalPage {
				pagination.HasNext = true
				pagination.NextPageUrl = urlPath + "?page=" + strconv.Itoa(pagination.CurrentPage+1) + "&per_page=" + strconv.Itoa(pagination.PerPage)
			}
		} else {
			start := time.Now()

			c.Request.ParseForm()
			c.Request.ParseMultipartForm(10000)

			// 读取 body，但不破坏后续使用
			var bodyBytes []byte
			if c.Request.Body != nil {
				bodyBytes, _ = io.ReadAll(c.Request.Body)
				// 重新赋值，让 BindJSON 等还能读
				c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			}

			// 处理 body：美化 + 解析
			var jsonParams url.Values
			if len(bodyBytes) > 0 {
				// 解析成 map 并打平为 url.Values
				var jsonMap map[string]interface{}
				if err := json.Unmarshal(bodyBytes, &jsonMap); err == nil {
					jsonParams = flattenJSONToValues(jsonMap, "")
				}
			}

			request := RequestStat{
				RequestedAt:   start,
				RequestUrl:    urlPath,
				HttpMethod:    c.Request.Method,
				HttpStatus:    c.Writer.Status(),
				ContentType:   c.ContentType(),
				Headers:       c.Request.Header,
				Cookies:       c.Request.Cookies(),
				GetParams:     c.Request.URL.Query(),
				PostParams:    c.Request.PostForm,
				Json:          jsonParams,
				PostMultipart: c.Request.MultipartForm,
				ClientIP:      c.ClientIP(),
			}

			allRequests.Requets = append([]RequestStat{request}, allRequests.Requets...)
		}

		c.Next()
	}
}

func paginate(s []RequestStat, offset, length int) []RequestStat {
	end := offset + length
	if end < len(s) {
		return s[offset:end]
	}

	return s[offset:]
}
func flattenJSONToValues(data map[string]interface{}, prefix string) url.Values {
	values := make(url.Values)

	for k, v := range data {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}

		switch val := v.(type) {
		case map[string]interface{}:
			// 递归处理嵌套对象
			for subK, subV := range flattenJSONToValues(val, key) {
				values[subK] = subV
			}
		case []interface{}:
			// 数组转成 "key[0]", "key[1]" 形式
			for i, item := range val {
				arrayKey := fmt.Sprintf("%s[%d]", key, i)
				if itemMap, ok := item.(map[string]interface{}); ok {
					for subK, subV := range flattenJSONToValues(itemMap, arrayKey) {
						values[subK] = subV
					}
				} else {
					values[arrayKey] = []string{fmt.Sprint(item)}
				}
			}
		default:
			values[key] = []string{fmt.Sprint(v)}
		}
	}
	return values
}
