package templates

import (
	"context"
	"strings"
	"watchAlert/internal/models"
	"watchAlert/pkg/tools"

	"github.com/bytedance/sonic"
	"github.com/zeromicro/go-zero/core/logc"
)

// Template 飞书消息卡片模版
func feishuTemplate(alert models.AlertCurEvent, noticeTmpl models.NoticeTemplateExample) string {

	var cardContentString string
	if *noticeTmpl.EnableFeiShuJsonCard {
		defaultTemplate := models.FeiShuJsonCardMsg{
			MsgType: "interactive",
		}
		var tmplC models.JsonCards
		switch alert.IsRecovered {
		case false:
			cardContentString = noticeTmpl.TemplateFiring
		case true:
			cardContentString = noticeTmpl.TemplateRecover
		}
		cardContentString = ParserJSONTemplate("Card", alert, cardContentString)
		if err := sonic.Unmarshal([]byte(cardContentString), &tmplC); err != nil {
			logc.Errorf(context.Background(), "飞书 JSON 卡片模板渲染失败: %v", err)
			return ""
		}
		defaultTemplate.Card = tmplC
		cardContentString = tools.JsonMarshalToString(defaultTemplate)

	} else {
		defaultTemplate := models.FeiShuJsonCardMsg{
			MsgType: "interactive",
			Card: models.JsonCards{
				Config: tools.ConvertStructToMap(models.Configs{
					EnableForward: true,
					WidthMode:     models.WidthModeDefault,
				}),
			},
		}
		cardHeader := models.Headers{
			Template: ParserTemplate("TitleColor", alert, noticeTmpl.Template),
			Title: models.Titles{
				Content: ParserTemplate("Title", alert, noticeTmpl.Template),
				Tag:     "plain_text",
			},
		}
		cardElements := []models.Elements{
			{
				Tag:            "column_set",
				FlexMode:       "none",
				BackgroupStyle: "default",
				Columns: []models.Columns{
					{
						Tag:           "column",
						Width:         "weighted",
						Weight:        1,
						VerticalAlign: "top",
						Elements: []models.ColumnsElements{
							{
								Tag: "div",
								Text: models.Texts{
									Content: ParserTemplate("Event", alert, noticeTmpl.Template),
									Tag:     "lark_md",
								},
							},
						},
					},
				},
			},
			{
				Tag: "hr",
			},
			{
				Tag: "note",
				Elements: []models.ElementsElements{
					{
						Tag:     "plain_text",
						Content: ParserTemplate("Footer", alert, noticeTmpl.Template),
					},
				},
			},
		}

		defaultTemplate.Card.Elements = tools.ConvertSliceToMapList(cardElements)
		defaultTemplate.Card.Header = tools.ConvertStructToMap(cardHeader)
		cardContentString = tools.JsonMarshalToString(defaultTemplate)

	}

	// 需要将所有换行符进行转义
	cardContentString = strings.Replace(cardContentString, "\n", "\\n", -1)

	return cardContentString

}
