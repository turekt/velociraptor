package reporting

import (
	"bytes"
	"context"
	"text/template"

	"github.com/olekukonko/tablewriter"
	config_proto "www.velocidex.com/golang/velociraptor/config/proto"
	"www.velocidex.com/golang/velociraptor/utils"
	"www.velocidex.com/golang/vfilter"
)

type TextTemplateEngine struct {
	*BaseTemplateEngine
	tmpl *template.Template
}

func (self *TextTemplateEngine) Execute(template_string string) (string, error) {
	tmpl, err := self.tmpl.Parse(template_string)
	if err != nil {
		return "", err
	}

	buffer := &bytes.Buffer{}
	err = tmpl.Execute(buffer, nil)
	if err != nil {
		utils.Debug(err)
		return "", err
	}

	return buffer.String(), nil
}

func (self *TextTemplateEngine) Query(queries ...string) []vfilter.Row {
	result := []vfilter.Row{}

	for _, query := range queries {
		t := self.tmpl.Lookup(query)
		if t != nil {
			buf := &bytes.Buffer{}
			err := t.Execute(buf, nil)
			if err != nil {
				self.logger.Err("Template Error (%s): %v",
					self.Artifact.Name, err)
				return []vfilter.Row{}
			}
			query = buf.String()
		}

		vql, err := vfilter.Parse(query)
		if err != nil {
			self.logger.Err("VQL Error while reporting %s: %v",
				self.Artifact.Name, err)
			return result
		}

		for row := range vql.Eval(context.Background(), self.Scope) {
			result = append(result, row)
		}
	}

	return result
}

// Not implemented for text.
func (self *TextTemplateEngine) LineChart(values ...interface{}) string {
	return self.Table(values...)
}

func (self *TextTemplateEngine) Table(values ...interface{}) string {
	_, argv := parseOptions(values)
	// Not enough args.
	if len(argv) != 1 {
		return ""
	}

	rows, ok := argv[0].([]vfilter.Row)
	if !ok { // Not the right type
		return ""
	}

	buffer := &bytes.Buffer{}
	table := tablewriter.NewWriter(buffer)

	if len(rows) == 0 {
		return ""
	}

	columns := self.Scope.GetMembers(rows[0])
	table.SetHeader(columns)

	for _, row := range rows {
		string_row := []string{}
		for _, key := range columns {
			cell := ""
			value, pres := self.Scope.Associative(row, key)
			if pres && !utils.IsNil(value) {
				cell = utils.Stringify(value, self.Scope, 120/len(columns))
			}
			string_row = append(string_row, cell)
		}
		table.Append(string_row)
	}

	table.Render()

	return buffer.String()
}

func NewTextTemplateEngine(config_obj *config_proto.Config,
	artifact_name string) (*TextTemplateEngine, error) {
	base_engine, err := newBaseTemplateEngine(
		config_obj, artifact_name)
	if err != nil {
		return nil, err
	}

	template_engine := &TextTemplateEngine{BaseTemplateEngine: base_engine}
	template_engine.tmpl = template.New("").Funcs(
		template.FuncMap{
			"Query":     template_engine.Query,
			"Scope":     template_engine.GetScope,
			"Table":     template_engine.Table,
			"LineChart": template_engine.LineChart,
			"Get":       template_engine.getFunction,
			"str":       strval,
		})

	return template_engine, nil
}
