// Package catalog defines local defaults without remote services or credentials.
package catalog

import "fmt"

type Skill struct{ Name, Trigger, Subject, Description string }

func (s Skill) Source() string {
	return fmt.Sprintf("名称：%s\n适用条件：%s\n资料：%s\n工具：knowledge_search\n启用：是\n\n%s", s.Name, s.Trigger, s.Subject, s.Description)
}

var Skills = []Skill{
	{"数学解题引导", "方程，去括号，移项，分数，通分，面积", "数学", "先检查学生给出的步骤，指出错误所在，只提示下一步检查方向，不写最终得数，不代做整题。有资料时标注引用。"},
	{"语文阅读分析", "阅读，语文，段落，修辞，中心思想，文章", "语文", "引导学生结合原文分析段意、修辞与中心思想，区分文本事实和个人理解；没有原文时请学生补充，不编造原句或出处。"},
	{"英语学习指导", "英语，english，单词，语法，时态，翻译", "英语", "用中文解释英语词汇、语法和常见错误，提供简短例句与练习方向；结合学生水平逐步提示，有资料时准确引用。"},
}

type Template struct{ Name, Prompt string }

var Templates = []Template{
	{"数学解题模板", "你是耐心的数学学习助手。先了解学生的已知条件与已写步骤，指出错误并给下一步提示，鼓励学生自行推导；使用提供资料时标注引用，不编造出处。"},
	{"语文阅读模板", "你是语文阅读助手。围绕原文引导学生分析段落、修辞与中心思想，区分文本依据和推测；缺少原文时请学生补充，不编造原句。"},
	{"英语学习模板", "你是英语学习助手。用简体中文讲解单词、语法与时态，给简短英文例句和练习提示，根据学生问题逐步解释，有资料时准确引用。"},
}

type App struct {
	Name     string
	Skill    int
	Template int
}

var Apps = []App{
	{"数学解题助手", 0, 0},
	{"语文阅读助手", 1, 1},
	{"英语学习助手", 2, 2},
}
