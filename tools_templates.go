package ahasendmcp

import (
	"context"
	"strings"
	"time"

	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
)

// ahamlSummary says what AhaML is and where its reference is. Each tool whose
// content takes or returns markup holds it, through ahamlReadNote,
// ahamlWriteNote or ahamlUpdateNote.
const ahamlSummary = "A Simple Editor design (editor simple, the dashboard's visual editor) is read and written " +
	"as AhaSend Markup (AhaML) in markup: XML with a tag for each part, such as " +
	`<Email version="1"><Heading level="1">Hi <Var name="first_name" fallback="there"/></Heading>` +
	`<Paragraph><Bold>Your order</Bold> has shipped.</Paragraph><Button url="https://example.com/orders">Track it</Button></Email>. ` +
	"A variable in text is a <Var> tag, not {{ }}. The AhaML reference is the resource " + ahamlReferenceURI +
	", also at https://ahasend.com/docs/send-api/ahaml."

// ahamlReadNote is the AhaML part of the description of a tool that returns
// content.
const ahamlReadNote = ahamlSummary + " content.markup holds the design as AhaML, which update tools take " +
	"back. When AhaML cannot be made, markup is missing and markup_error says why, such as for a design with " +
	"an HTML block."

// ahamlWriteNote is the AhaML part of the description of a tool that creates
// a design.
const ahamlWriteNote = ahamlSummary + " markup sets the whole design; what it leaves out takes its default. " +
	"It is only for a Simple Editor design; where no editor is set yet, as on a create, it means editor simple. " +
	"html beside markup is ignored; mjml beside it is refused. Markup sent back as read is checked too: a " +
	"design that breaks an AhaML rule, such as an http: image, is refused, naming the block. A refusal lists " +
	"up to 100 problems, with line, column and path; its message gives the total. Warnings come back in " +
	"markup_warnings."

// ahamlUpdateNote is the AhaML part of the description of a tool that changes
// a design.
const ahamlUpdateNote = ahamlWriteNote + " A dashboard editor that was open before this write can save over it."

// The campaign template tools also require campaigns:read: their endpoints
// exist only for an account with marketing on, and only such an account's
// keys can hold campaigns:read, so a connection without marketing does not
// list them.
func templatesTools() []tool {
	return []tool{
		newTool(ToolInfo{
			Name:           "list_templates",
			Title:          "List templates",
			Toolset:        ToolsetTemplates,
			RequiredScopes: []string{"templates:read"},
			Annotations:    annRead(),
		}, "Lists the transactional templates of this account, newest first, with their name, editor, "+
			"subject, sender and whether they have unpublished changes, but not their content. Use it to find a "+
			"template for send_template_email or to edit; get_published_template returns the copy a send uses, and get_template the one to edit. "+
			"The result holds one page and a next_cursor for the page after it.",
			listTemplates),
		newTool(ToolInfo{
			Name:           "get_template",
			Title:          "Get template",
			Toolset:        ToolsetTemplates,
			RequiredScopes: []string{"templates:read"},
			Annotations:    annRead(),
		}, "Returns one transactional template's working copy, with its subject, sender, variables and "+
			"content: its draft while one exists (has_draft true), and its published copy otherwise. Use it to "+
			"read a template to edit; update_template takes its content back. To send the template, read "+
			"get_published_template, which holds the variables a send must fill. "+variableKindsNote+" "+ahamlReadNote,
			getTemplate),
		newTool(ToolInfo{
			Name:           "get_published_template",
			Title:          "Get published template",
			Toolset:        ToolsetTemplates,
			RequiredScopes: []string{"templates:read"},
			Annotations:    annRead(),
		}, "Returns one transactional template as sends use it (its published copy): its subject, sender, "+
			"content and the variables a send must fill. Use it before sending the template. has_draft true "+
			"means unpublished changes exist, which get_template returns. A template never published has no "+
			"content. "+variableKindsNote+" "+ahamlReadNote,
			getPublishedTemplate),
		newTool(ToolInfo{
			Name:           "create_template",
			Title:          "Create template",
			Toolset:        ToolsetTemplates,
			RequiredScopes: []string{"templates:write"},
			Annotations:    annWrite(),
		}, "Creates a transactional template from AhaML, HTML or MJML, with an optional plain text version. Use "+
			"it to build a template that send_template_email sends. The content goes to the template's draft, and "+
			"sends fail until it is published: set publish to true, or call publish_template later. Without "+
			"text, the text is made from the HTML. The result is the template as written, its draft unless "+
			"publish was set, with its variables but without its content; get_template returns the content. "+
			ahamlWriteNote,
			createTemplate),
		newTool(ToolInfo{
			Name:           "update_template",
			Title:          "Update template",
			Toolset:        ToolsetTemplates,
			RequiredScopes: []string{"templates:write"},
			Annotations:    annWrite(),
		}, "Changes a transactional template. Use it to edit a template's subject, sender or content. Only "+
			"the fields given change. The name changes at once; every "+
			"other field goes to the template's draft, the same draft the dashboard edits, so sends keep the "+
			"published copy until the draft is published (publish true, or publish_template). The editor "+
			"cannot change: give markup for a Simple Editor template, mjml for an advanced template and html for an "+
			"html template. Read the content to edit from get_template. The result is the template as written, "+
			"its draft while one exists, with its variables but without its content; get_template returns the "+
			"content. "+ahamlUpdateNote,
			updateTemplate),
		newTool(ToolInfo{
			Name:           "delete_template",
			Title:          "Delete template",
			Toolset:        ToolsetTemplates,
			RequiredScopes: []string{"templates:delete"},
			Annotations:    annDestructive(),
		}, "Deletes a transactional template; sends of it fail from then on. Use it only when the user asks to "+
			"remove the template. The result is AhaSend's confirmation.",
			deleteTemplate),
		newTool(ToolInfo{
			Name:           "discard_template_draft",
			Title:          "Discard template draft",
			Toolset:        ToolsetTemplates,
			RequiredScopes: []string{"templates:write"},
			Annotations:    annDestructive(),
		}, "Discards a transactional template's unpublished draft, including changes made in the dashboard; the "+
			"published copy stays. Use it only when the user wants those changes dropped. The result is the "+
			"template, without its content.",
			discardTemplateDraft),
		newTool(ToolInfo{
			Name:           "publish_template",
			Title:          "Publish template",
			Toolset:        ToolsetTemplates,
			RequiredScopes: []string{"templates:write"},
			Annotations:    annWrite(),
		}, "Publishes a transactional template's whole draft, including changes made in the dashboard, so sends "+
			"use it, and keeps it as a new version. Use it once the draft is ready. A template without a draft "+
			"is left as it is when its published copy has a body, and refused when it has none. The result is "+
			"the published template with its variables, without its content.",
			publishTemplate),
		newTool(ToolInfo{
			Name:           "list_template_versions",
			Title:          "List template versions",
			Toolset:        ToolsetTemplates,
			RequiredScopes: []string{"templates:read"},
			Annotations:    annRead(),
		}, "Lists the published versions of a transactional template, newest first, without their content: "+
			"every version AhaSend keeps (at most 50), in one result. Use it to find a version to read with "+
			"get_template_version or to restore.",
			listTemplateVersions),
		newTool(ToolInfo{
			Name:           "get_template_version",
			Title:          "Get template version",
			Toolset:        ToolsetTemplates,
			RequiredScopes: []string{"templates:read"},
			Annotations:    annRead(),
		}, "Returns one published version of a transactional template with its subject, sender, variables and "+
			"content. Use it to compare an old version or before restoring it. "+variableKindsNote+" "+ahamlReadNote,
			getTemplateVersion),
		newTool(ToolInfo{
			Name:           "restore_template_version",
			Title:          "Restore template version",
			Toolset:        ToolsetTemplates,
			RequiredScopes: []string{"templates:write"},
			Annotations:    annWrite(),
		}, "Copies a published version of a transactional template into its draft, replacing the draft. Use it "+
			"to go back to an earlier version. Sends keep the published copy until the draft is published: set "+
			"publish to true to publish it at once. The result is the template, without its content.",
			restoreTemplateVersion),
		newTool(ToolInfo{
			Name:           "list_campaign_templates",
			Title:          "List campaign templates",
			Toolset:        ToolsetTemplates,
			RequiredScopes: []string{"templates:read", "campaigns:read"},
			Annotations:    annRead(),
		}, "Lists the campaign templates of this account, newest first, with their name and editor, but not "+
			"their content. A campaign template is a design a campaign copies when it is made from it; "+
			"transactional templates are not listed here. Use it to pick a design for create_campaign. The "+
			"result holds one page and a next_cursor for the page after it.",
			listCampaignTemplates),
		newTool(ToolInfo{
			Name:           "get_campaign_template",
			Title:          "Get campaign template",
			Toolset:        ToolsetTemplates,
			RequiredScopes: []string{"templates:read", "campaigns:read"},
			Annotations:    annRead(),
		}, "Returns one campaign template with its content. Use it to read or edit the design. A campaign can be "+
			"made from it only once it has HTML. "+ahamlReadNote,
			getCampaignTemplate),
		newTool(ToolInfo{
			Name:           "create_campaign_template",
			Title:          "Create campaign template",
			Toolset:        ToolsetTemplates,
			RequiredScopes: []string{"templates:write", "campaigns:read"},
			Annotations:    annWrite(),
		}, "Creates a campaign template, a reusable design for campaigns, from AhaML, HTML or MJML with an "+
			"optional plain text version. Use it for a design several campaigns share; a campaign copies the "+
			"design when it is made, and later changes to the template do not reach it. It has no draft and no "+
			"subject or sender; the campaign owns those. The result is the campaign template, without its "+
			"content. "+ahamlWriteNote,
			createCampaignTemplate),
		newTool(ToolInfo{
			Name:           "update_campaign_template",
			Title:          "Update campaign template",
			Toolset:        ToolsetTemplates,
			RequiredScopes: []string{"templates:write", "campaigns:read"},
			Annotations:    annWrite(),
		}, "Changes a campaign template's name, content or both, at once. Use it to edit a shared campaign "+
			"design. Only the fields given change. "+
			"Campaigns already made from it keep their own copies. The editor cannot change: give markup for a "+
			"Simple Editor design, mjml for an advanced design and html for an html design. The result is the "+
			"campaign template, without its content. "+ahamlUpdateNote,
			updateCampaignTemplate),
		newTool(ToolInfo{
			Name:           "delete_campaign_template",
			Title:          "Delete campaign template",
			Toolset:        ToolsetTemplates,
			RequiredScopes: []string{"templates:delete", "campaigns:read"},
			Annotations:    annDestructive(),
		}, "Deletes a campaign template. Campaigns made from it keep their own copies. Use it only when the user "+
			"asks to remove the template. The result is AhaSend's confirmation.",
			deleteCampaignTemplate),
	}
}

// Shared template types.

// senderOutput is a sender address.
type senderOutput struct {
	Email string `json:"email" jsonschema:"Email address."`
	Name  string `json:"name,omitempty" jsonschema:"Display name."`
}

func senderFrom(a *common.SenderAddress) *senderOutput {
	if a == nil || a.Email == "" {
		return nil
	}
	return &senderOutput{Email: a.Email, Name: deref(a.Name)}
}

func replyToFrom(a *common.SenderAddress) string {
	if a == nil {
		return ""
	}
	return a.Email
}

// variableKindsNote tells an agent where to find what a template variable
// takes.
const variableKindsNote = "Each variable's kind tells what to send for it, and a list variable takes an array of " +
	"objects with its item_fields as keys."

// templateVariable is a variable a template's design uses.
type templateVariable struct {
	Name     string `json:"name" jsonschema:"Variable name, as substitutions name it."`
	Required bool   `json:"required" jsonschema:"True when a send must give a value for it."`
	Kind     string `json:"kind" jsonschema:"What to send for it. value: text or a number that the email shows. condition: it only decides whether a part of the email shows; send a non-empty value to show the part, and omit it or send \"\", false, 0, null, [] or {} to hide it (the strings \"false\" and \"0\" are not empty); if the design compares it, send a value that meets the comparison. list: an array, and a part of the email repeats once for each element. Any other value is a kind this server does not know: read the content to see how the design uses the variable."`
	// ItemFields is set only for a list, and is empty rather than absent
	// when the repeated part uses nothing from its elements.
	ItemFields *[]string `json:"item_fields,omitempty" jsonschema:"Only for a list: the keys of each element. Send each element as an object with these keys, such as [{\"name\": \"Linen shirt\", \"price\": \"49.00\"}]. When it is [\"value\"], plain values also work, such as [\"new\", \"sale\"]. When it is empty, send one element of any type for each repeat."`
}

func templateVariablesFrom(vars []responses.TemplateVariable) []templateVariable {
	out := make([]templateVariable, 0, len(vars))
	for _, v := range vars {
		tv := templateVariable{Name: v.Name, Required: v.Required, Kind: v.Kind}
		// A kind this module does not know passes through unchanged, so the
		// agent can read the content to see what the design wants.
		switch v.Kind {
		case "":
			// An API without kinds treats every variable as a value.
			tv.Kind = responses.TemplateVariableKindValue
		case responses.TemplateVariableKindList:
			fields := append([]string{}, v.ItemFields...)
			tv.ItemFields = &fields
		}
		out = append(out, tv)
	}
	return out
}

// templateContent is the content of a template, version or campaign.
type templateContent struct {
	MJML         string       `json:"mjml,omitempty" jsonschema:"MJML source of an advanced design."`
	HTML         string       `json:"html,omitempty" jsonschema:"HTML of an html design, or the HTML made from a Simple Editor design, which a write changes through markup. An advanced design returns mjml instead."`
	Markup       string       `json:"markup,omitempty" jsonschema:"AhaML of a Simple Editor design, which markup in a write takes back. The resource ahasend://docs/ahaml is its reference."`
	MarkupError  *markupError `json:"markup_error,omitempty" jsonschema:"Why a Simple Editor design has no markup; missing when markup is there."`
	Text         string       `json:"text,omitempty" jsonschema:"Plain text version."`
	TextIsCustom bool         `json:"text_is_custom" jsonschema:"True when the text was written rather than made from the HTML."`
}

func templateContentFrom(c *responses.TemplateContent) *templateContent {
	if c == nil {
		return nil
	}
	out := &templateContent{MJML: c.MJML, HTML: c.HTML, Markup: deref(c.Markup), Text: c.Text, TextIsCustom: c.TextIsCustom}
	if c.MarkupError != nil {
		out.MarkupError = &markupError{Code: c.MarkupError.Code, Message: c.MarkupError.Message}
	}
	return out
}

// markupError tells why a Simple Editor design has no AhaML.
type markupError struct {
	Code    string `json:"code" jsonschema:"no_design (the design has no blocks yet), html_block (it holds an HTML block, which AhaML cannot show; remove the block in the dashboard), unmapped_block (it holds a block AhaML has no tag for), bad_stored_value (it holds a value AhaML cannot write), renderer_unavailable or rate_limited (read it again later)."`
	Message string `json:"message" jsonschema:"What went wrong."`
}

// markupWarning is a warning about the AhaML a write stored.
type markupWarning struct {
	Code    string `json:"code" jsonschema:"Warning code, such as jinja_in_text: text holds {{ }}, which stays text; a variable in text is a <Var> tag."`
	Message string `json:"message" jsonschema:"What the warning is about."`
	Line    int    `json:"line,omitempty" jsonschema:"Line of the markup where it starts, counted from 1."`
	Column  int    `json:"column,omitempty" jsonschema:"Column where it starts, counted from 1 in characters."`
	Path    string `json:"path,omitempty" jsonschema:"The element, such as Email > Section[2] > Paragraph[1]."`
}

func markupWarningsFrom(problems []common.MarkupProblem) []markupWarning {
	if len(problems) == 0 {
		return nil
	}
	out := make([]markupWarning, 0, len(problems))
	for _, p := range problems {
		out = append(out, markupWarning{Code: p.Code, Message: p.Message, Line: p.Line, Column: p.Column, Path: p.Path})
	}
	return out
}

// contentInput is the content of a template or campaign write.
type contentInput struct {
	MJML string `json:"mjml,omitempty" jsonschema:"MJML source, for an advanced design. Not with markup or html. Images and stylesheets must use https URLs."`
	HTML string `json:"html,omitempty" jsonschema:"HTML, for an html design. Not with mjml; beside markup it is ignored. Images and stylesheets must use https URLs."`
	// Markup is a pointer so that null is no markup and "" is sent, as the
	// API takes them.
	Markup *string `json:"markup,omitempty" jsonschema:"AhaML, for a Simple Editor design. Not with mjml; null is the same as no markup. Images must use https URLs. The resource ahasend://docs/ahaml is its reference."`
	Text   string  `json:"text,omitempty" jsonschema:"Plain text version. When omitted, it is made from the HTML."`
}

func (c contentInput) request() *requests.TemplateContentInput {
	if c.MJML == "" && c.HTML == "" && c.Markup == nil && c.Text == "" {
		return nil
	}
	return &requests.TemplateContentInput{
		MJML:   optionalString(c.MJML),
		HTML:   optionalString(c.HTML),
		Markup: c.Markup,
		Text:   optionalString(c.Text),
	}
}

// contentUpdateInput is the content of a template or campaign update.
type contentUpdateInput struct {
	MJML         string  `json:"mjml,omitempty" jsonschema:"New MJML source, for an advanced design. Omit to keep the design."`
	HTML         string  `json:"html,omitempty" jsonschema:"New HTML, for an html design. Omit to keep the design; beside markup it is ignored."`
	Markup       *string `json:"markup,omitempty" jsonschema:"New AhaML, for a Simple Editor design; it replaces the whole design. Omit it, or give null, to keep the design. The resource ahasend://docs/ahaml is its reference."`
	Text         string  `json:"text,omitempty" jsonschema:"New plain text version. Omit to keep the text; beside new markup, mjml or html, a text made from the old HTML is made again from the new one, and a written text is kept."`
	TextFromHTML bool    `json:"text_from_html,omitempty" jsonschema:"True to replace the text with one made from the HTML. Not with text."`
}

func (c contentUpdateInput) request() (*requests.TemplateContentInput, error) {
	if c.TextFromHTML && c.Text != "" {
		return nil, invalidInput("Give text or text_from_html, not both.")
	}
	in := contentInput{MJML: c.MJML, HTML: c.HTML, Markup: c.Markup, Text: c.Text}.request()
	if c.TextFromHTML {
		if in == nil {
			in = &requests.TemplateContentInput{}
		}
		// The API makes the text again from the HTML for a text sent as null.
		in.Text = ptr("")
	}
	return in, nil
}

// addressUpdate is a sender address change: nil keeps the address, and an
// empty email clears it, which ahasend-go sends as null.
func addressUpdate(a *addressInput) *common.SenderAddress {
	if a == nil {
		return nil
	}
	if strings.TrimSpace(a.Email) == "" {
		return &common.SenderAddress{}
	}
	return senderPtr(a)
}

// replyToUpdate is a reply-to address change: nil keeps it, and an empty
// value clears it.
func replyToUpdate(email *string) *common.SenderAddress {
	if email == nil {
		return nil
	}
	if strings.TrimSpace(*email) == "" {
		return &common.SenderAddress{}
	}
	return &common.SenderAddress{Email: strings.TrimSpace(*email)}
}

// replyToAddress is a reply-to address without a display name.
func replyToAddress(email string) *common.SenderAddress {
	email = strings.TrimSpace(email)
	if email == "" {
		return nil
	}
	return &common.SenderAddress{Email: email}
}

// Transactional templates.

// templateSummary is a template without its content.
type templateSummary struct {
	ID        string        `json:"id" jsonschema:"Template ID."`
	Name      string        `json:"name" jsonschema:"Template name."`
	Editor    string        `json:"editor" jsonschema:"advanced (MJML), html, or simple (the Simple Editor, written here as AhaML in markup)."`
	Subject   string        `json:"subject,omitempty" jsonschema:"Subject of this copy."`
	Preheader string        `json:"preheader,omitempty" jsonschema:"Preview text of this copy."`
	From      *senderOutput `json:"from,omitempty" jsonschema:"Default sender; missing when every send must name one."`
	ReplyTo   string        `json:"reply_to,omitempty" jsonschema:"Default reply-to address."`
	HasDraft  bool          `json:"has_draft" jsonschema:"True when changes exist that are not published yet."`
	CreatedAt time.Time     `json:"created_at" jsonschema:"When the template was created."`
	UpdatedAt time.Time     `json:"updated_at" jsonschema:"When this copy or the name last changed."`
}

func templateSummaryFrom(t *responses.Template) templateSummary {
	return templateSummary{
		ID:        t.ID.String(),
		Name:      t.Name,
		Editor:    t.Editor,
		Subject:   t.Subject,
		Preheader: t.Preheader,
		From:      senderFrom(t.From),
		ReplyTo:   replyToFrom(t.ReplyTo),
		HasDraft:  t.HasDraft,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
}

// templateResult is a template as a write returns it, its working copy: with
// its variables, but without its content.
type templateResult struct {
	templateSummary
	Variables      []templateVariable `json:"variables" jsonschema:"Variables this copy's design uses."`
	MarkupWarnings []markupWarning    `json:"markup_warnings,omitempty" jsonschema:"Warnings about the markup this write stored; only a write with markup has them."`
}

func templateResultFrom(t *responses.Template) templateResult {
	return templateResult{
		templateSummary: templateSummaryFrom(t),
		Variables:       templateVariablesFrom(t.Variables),
		MarkupWarnings:  markupWarningsFrom(t.MarkupWarnings),
	}
}

// templateDetail is a template copy with its content.
type templateDetail struct {
	templateResult
	Content *templateContent `json:"content,omitempty" jsonschema:"Content of this copy; missing when it has no body."`
}

type listTemplatesOutput struct {
	Templates []templateSummary `json:"templates" jsonschema:"This page's templates."`
	pageOutput
}

func listTemplates(ctx context.Context, env *toolEnv, in pageInput) (listTemplatesOutput, error) {
	page, err := in.params()
	if err != nil {
		return listTemplatesOutput{}, err
	}
	resp, _, err := env.client.TemplatesAPI.GetTemplates(ctx, env.accountID, requests.GetTemplatesParams{
		Limit: page.Limit,
		After: page.After,
	})
	if err != nil {
		return listTemplatesOutput{}, err
	}
	out := listTemplatesOutput{Templates: []templateSummary{}, pageOutput: pageOutputFrom(resp.Pagination)}
	for i := range resp.Data {
		out.Templates = append(out.Templates, templateSummaryFrom(&resp.Data[i]))
	}
	return out, nil
}

type templateIDInput struct {
	TemplateID string `json:"template_id" jsonschema:"Template ID."`
}

func getTemplate(ctx context.Context, env *toolEnv, in templateIDInput) (templateDetail, error) {
	id, err := parseID("template_id", in.TemplateID)
	if err != nil {
		return templateDetail{}, err
	}
	t, _, err := env.client.TemplatesAPI.GetTemplate(ctx, env.accountID, id)
	if err != nil {
		return templateDetail{}, err
	}
	return templateDetail{templateResult: templateResultFrom(t), Content: templateContentFrom(t.Content)}, nil
}

func getPublishedTemplate(ctx context.Context, env *toolEnv, in templateIDInput) (templateDetail, error) {
	id, err := parseID("template_id", in.TemplateID)
	if err != nil {
		return templateDetail{}, err
	}
	t, _, err := env.client.TemplatesAPI.GetPublishedTemplate(ctx, env.accountID, id)
	if err != nil {
		return templateDetail{}, err
	}
	return templateDetail{templateResult: templateResultFrom(t), Content: templateContentFrom(t.Content)}, nil
}

type createTemplateInput struct {
	Name      string        `json:"name" jsonschema:"Template name."`
	Editor    string        `json:"editor,omitempty" jsonschema:"advanced, html or simple. Optional with markup (simple), mjml (advanced) or html (html); simple alone makes an empty Simple Editor design."`
	Subject   string        `json:"subject,omitempty" jsonschema:"Subject."`
	Preheader string        `json:"preheader,omitempty" jsonschema:"Preview text."`
	From      *addressInput `json:"from,omitempty" jsonschema:"Default sender, on a domain of this account with valid DNS records. Omit to make every send name one."`
	ReplyTo   string        `json:"reply_to,omitempty" jsonschema:"Default reply-to address; needs from."`
	contentInput
	Publish bool `json:"publish,omitempty" jsonschema:"True to publish the template at once, so sends can use it."`
}

func createTemplate(ctx context.Context, env *toolEnv, in createTemplateInput) (templateResult, error) {
	req := requests.CreateTemplateRequest{
		Name:      in.Name,
		Editor:    optionalString(in.Editor),
		Subject:   optionalString(in.Subject),
		Preheader: optionalString(in.Preheader),
		From:      senderPtr(in.From),
		ReplyTo:   replyToAddress(in.ReplyTo),
		Content:   in.request(),
		Publish:   in.Publish,
	}
	t, _, err := env.client.TemplatesAPI.CreateTemplate(ctx, env.accountID, req)
	if err != nil {
		return templateResult{}, err
	}
	return templateResultFrom(t), nil
}

type updateTemplateInput struct {
	TemplateID string        `json:"template_id" jsonschema:"Template ID."`
	Name       string        `json:"name,omitempty" jsonschema:"New name."`
	Subject    *string       `json:"subject,omitempty" jsonschema:"New subject. An empty value clears it."`
	Preheader  *string       `json:"preheader,omitempty" jsonschema:"New preview text. An empty value clears it."`
	From       *addressInput `json:"from,omitempty" jsonschema:"New default sender, address and name together. An empty email clears the sender and the reply-to."`
	ReplyTo    *string       `json:"reply_to,omitempty" jsonschema:"New default reply-to address. An empty value clears it."`
	contentUpdateInput
	Publish bool `json:"publish,omitempty" jsonschema:"True to publish the whole draft after the change. With no other field, the call only publishes."`
}

func updateTemplate(ctx context.Context, env *toolEnv, in updateTemplateInput) (templateResult, error) {
	id, err := parseID("template_id", in.TemplateID)
	if err != nil {
		return templateResult{}, err
	}
	content, err := in.request()
	if err != nil {
		return templateResult{}, err
	}
	req := requests.UpdateTemplateRequest{
		Name:      optionalString(in.Name),
		Subject:   in.Subject,
		Preheader: in.Preheader,
		From:      addressUpdate(in.From),
		ReplyTo:   replyToUpdate(in.ReplyTo),
		Content:   content,
		Publish:   in.Publish,
	}
	t, _, err := env.client.TemplatesAPI.UpdateTemplate(ctx, env.accountID, id, req)
	if err != nil {
		return templateResult{}, err
	}
	return templateResultFrom(t), nil
}

func deleteTemplate(ctx context.Context, env *toolEnv, in templateIDInput) (confirmationOutput, error) {
	id, err := parseID("template_id", in.TemplateID)
	if err != nil {
		return confirmationOutput{}, err
	}
	resp, _, err := env.client.TemplatesAPI.DeleteTemplate(ctx, env.accountID, id)
	if err != nil {
		return confirmationOutput{}, err
	}
	return confirmationOutput{Message: resp.Message}, nil
}

func discardTemplateDraft(ctx context.Context, env *toolEnv, in templateIDInput) (templateResult, error) {
	id, err := parseID("template_id", in.TemplateID)
	if err != nil {
		return templateResult{}, err
	}
	t, _, err := env.client.TemplatesAPI.DiscardTemplateDraft(ctx, env.accountID, id)
	if err != nil {
		return templateResult{}, err
	}
	return templateResultFrom(t), nil
}

func publishTemplate(ctx context.Context, env *toolEnv, in templateIDInput) (templateResult, error) {
	id, err := parseID("template_id", in.TemplateID)
	if err != nil {
		return templateResult{}, err
	}
	t, _, err := env.client.TemplatesAPI.PublishTemplate(ctx, env.accountID, id)
	if err != nil {
		return templateResult{}, err
	}
	return templateResultFrom(t), nil
}

// templatePublisher tells who published a version.
type templatePublisher struct {
	Type string `json:"type" jsonschema:"user for the dashboard, api_key for the API."`
	ID   string `json:"id" jsonschema:"The user's or the API key's ID."`
}

// templateVersionSummary is a published version without its content.
type templateVersionSummary struct {
	ID          string             `json:"id" jsonschema:"Version ID."`
	Version     int                `json:"version" jsonschema:"Version number, counted from 1."`
	PublishedAt time.Time          `json:"published_at" jsonschema:"When it was published."`
	PublishedBy *templatePublisher `json:"published_by,omitempty" jsonschema:"Who published it, when known."`
}

func templatePublisherFrom(p *responses.TemplatePublisher) *templatePublisher {
	if p == nil {
		return nil
	}
	return &templatePublisher{Type: p.Type, ID: p.ID.String()}
}

type listTemplateVersionsOutput struct {
	Versions []templateVersionSummary `json:"versions" jsonschema:"Every kept version, newest first."`
}

func listTemplateVersions(ctx context.Context, env *toolEnv, in templateIDInput) (listTemplateVersionsOutput, error) {
	id, err := parseID("template_id", in.TemplateID)
	if err != nil {
		return listTemplateVersionsOutput{}, err
	}
	resp, _, err := env.client.TemplatesAPI.GetTemplateVersions(ctx, env.accountID, id)
	if err != nil {
		return listTemplateVersionsOutput{}, err
	}
	out := listTemplateVersionsOutput{Versions: []templateVersionSummary{}}
	for _, v := range resp.Data {
		out.Versions = append(out.Versions, templateVersionSummary{
			ID:          v.ID.String(),
			Version:     v.Version,
			PublishedAt: v.PublishedAt,
			PublishedBy: templatePublisherFrom(v.PublishedBy),
		})
	}
	return out, nil
}

type templateVersionInput struct {
	TemplateID string `json:"template_id" jsonschema:"Template ID."`
	VersionID  string `json:"version_id" jsonschema:"Version ID, as list_template_versions returns it."`
}

func (in templateVersionInput) ids() (templateID, versionID uuid.UUID, err error) {
	if templateID, err = parseID("template_id", in.TemplateID); err != nil {
		return
	}
	versionID, err = parseID("version_id", in.VersionID)
	return
}

// templateVersionDetail is a published version with its content.
type templateVersionDetail struct {
	templateVersionSummary
	Subject   string             `json:"subject,omitempty" jsonschema:"Subject."`
	Preheader string             `json:"preheader,omitempty" jsonschema:"Preview text."`
	From      *senderOutput      `json:"from,omitempty" jsonschema:"Default sender."`
	ReplyTo   string             `json:"reply_to,omitempty" jsonschema:"Default reply-to address."`
	Variables []templateVariable `json:"variables" jsonschema:"Variables the version uses."`
	Content   *templateContent   `json:"content,omitempty" jsonschema:"Content; missing when the version has no body."`
}

func getTemplateVersion(ctx context.Context, env *toolEnv, in templateVersionInput) (templateVersionDetail, error) {
	templateID, versionID, err := in.ids()
	if err != nil {
		return templateVersionDetail{}, err
	}
	v, _, err := env.client.TemplatesAPI.GetTemplateVersion(ctx, env.accountID, templateID, versionID)
	if err != nil {
		return templateVersionDetail{}, err
	}
	return templateVersionDetail{
		templateVersionSummary: templateVersionSummary{
			ID:          v.ID.String(),
			Version:     v.Version,
			PublishedAt: v.PublishedAt,
			PublishedBy: templatePublisherFrom(v.PublishedBy),
		},
		Subject:   v.Subject,
		Preheader: v.Preheader,
		From:      senderFrom(v.From),
		ReplyTo:   replyToFrom(v.ReplyTo),
		Variables: templateVariablesFrom(v.Variables),
		Content:   templateContentFrom(v.Content),
	}, nil
}

type restoreTemplateVersionInput struct {
	templateVersionInput
	Publish bool `json:"publish,omitempty" jsonschema:"True to publish the draft once the version is restored into it."`
}

func restoreTemplateVersion(ctx context.Context, env *toolEnv, in restoreTemplateVersionInput) (templateResult, error) {
	templateID, versionID, err := in.ids()
	if err != nil {
		return templateResult{}, err
	}
	t, _, err := env.client.TemplatesAPI.RestoreTemplateVersion(ctx, env.accountID, templateID, versionID,
		requests.RestoreTemplateVersionRequest{Publish: in.Publish})
	if err != nil {
		return templateResult{}, err
	}
	return templateResultFrom(t), nil
}

// Campaign templates.

// campaignTemplateSummary is a campaign template without its content.
type campaignTemplateSummary struct {
	ID        string    `json:"id" jsonschema:"Campaign template ID, for create_campaign's template_id."`
	Name      string    `json:"name" jsonschema:"Campaign template name."`
	Editor    string    `json:"editor" jsonschema:"advanced (MJML), html, or simple (the Simple Editor, written here as AhaML in markup)."`
	CreatedAt time.Time `json:"created_at" jsonschema:"When it was created."`
	UpdatedAt time.Time `json:"updated_at" jsonschema:"When it last changed."`
}

func campaignTemplateSummaryFrom(t *responses.CampaignTemplate) campaignTemplateSummary {
	return campaignTemplateSummary{
		ID:        t.ID.String(),
		Name:      t.Name,
		Editor:    t.Editor,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
}

// campaignTemplateResult is a campaign template as a write returns it:
// without its content, with the warnings about the markup it stored.
type campaignTemplateResult struct {
	campaignTemplateSummary
	MarkupWarnings []markupWarning `json:"markup_warnings,omitempty" jsonschema:"Warnings about the markup this write stored; only a write with markup has them."`
}

func campaignTemplateResultFrom(t *responses.CampaignTemplate) campaignTemplateResult {
	return campaignTemplateResult{
		campaignTemplateSummary: campaignTemplateSummaryFrom(t),
		MarkupWarnings:          markupWarningsFrom(t.MarkupWarnings),
	}
}

// campaignTemplateDetail is a campaign template with its content.
type campaignTemplateDetail struct {
	campaignTemplateSummary
	Content *templateContent `json:"content,omitempty" jsonschema:"Content; missing when it has neither HTML nor text."`
}

type listCampaignTemplatesOutput struct {
	CampaignTemplates []campaignTemplateSummary `json:"campaign_templates" jsonschema:"This page's campaign templates."`
	pageOutput
}

func listCampaignTemplates(ctx context.Context, env *toolEnv, in pageInput) (listCampaignTemplatesOutput, error) {
	page, err := in.params()
	if err != nil {
		return listCampaignTemplatesOutput{}, err
	}
	resp, _, err := env.client.CampaignTemplatesAPI.ListCampaignTemplates(ctx, env.accountID,
		requests.ListCampaignTemplatesParams{Limit: page.Limit, After: page.After})
	if err != nil {
		return listCampaignTemplatesOutput{}, err
	}
	out := listCampaignTemplatesOutput{CampaignTemplates: []campaignTemplateSummary{}, pageOutput: pageOutputFrom(resp.Pagination)}
	for i := range resp.Data {
		out.CampaignTemplates = append(out.CampaignTemplates, campaignTemplateSummaryFrom(&resp.Data[i]))
	}
	return out, nil
}

type campaignTemplateIDInput struct {
	CampaignTemplateID string `json:"campaign_template_id" jsonschema:"Campaign template ID."`
}

func getCampaignTemplate(ctx context.Context, env *toolEnv, in campaignTemplateIDInput) (campaignTemplateDetail, error) {
	id, err := parseID("campaign_template_id", in.CampaignTemplateID)
	if err != nil {
		return campaignTemplateDetail{}, err
	}
	t, _, err := env.client.CampaignTemplatesAPI.GetCampaignTemplate(ctx, env.accountID, id)
	if err != nil {
		return campaignTemplateDetail{}, err
	}
	return campaignTemplateDetail{campaignTemplateSummary: campaignTemplateSummaryFrom(t), Content: templateContentFrom(t.Content)}, nil
}

type createCampaignTemplateInput struct {
	Name   string `json:"name" jsonschema:"Campaign template name."`
	Editor string `json:"editor,omitempty" jsonschema:"advanced, html or simple. Optional with markup (simple), mjml (advanced) or html (html); without content the template gets an empty design of this editor."`
	contentInput
}

func createCampaignTemplate(ctx context.Context, env *toolEnv, in createCampaignTemplateInput) (campaignTemplateResult, error) {
	t, _, err := env.client.CampaignTemplatesAPI.CreateCampaignTemplate(ctx, env.accountID, requests.CreateCampaignTemplateRequest{
		Name:    in.Name,
		Editor:  optionalString(in.Editor),
		Content: in.request(),
	})
	if err != nil {
		return campaignTemplateResult{}, err
	}
	return campaignTemplateResultFrom(t), nil
}

type updateCampaignTemplateInput struct {
	CampaignTemplateID string `json:"campaign_template_id" jsonschema:"Campaign template ID."`
	Name               string `json:"name,omitempty" jsonschema:"New name."`
	contentUpdateInput
}

func updateCampaignTemplate(ctx context.Context, env *toolEnv, in updateCampaignTemplateInput) (campaignTemplateResult, error) {
	id, err := parseID("campaign_template_id", in.CampaignTemplateID)
	if err != nil {
		return campaignTemplateResult{}, err
	}
	content, err := in.request()
	if err != nil {
		return campaignTemplateResult{}, err
	}
	t, _, err := env.client.CampaignTemplatesAPI.UpdateCampaignTemplate(ctx, env.accountID, id, requests.UpdateCampaignTemplateRequest{
		Name:    optionalString(in.Name),
		Content: content,
	})
	if err != nil {
		return campaignTemplateResult{}, err
	}
	return campaignTemplateResultFrom(t), nil
}

func deleteCampaignTemplate(ctx context.Context, env *toolEnv, in campaignTemplateIDInput) (confirmationOutput, error) {
	id, err := parseID("campaign_template_id", in.CampaignTemplateID)
	if err != nil {
		return confirmationOutput{}, err
	}
	resp, _, err := env.client.CampaignTemplatesAPI.DeleteCampaignTemplate(ctx, env.accountID, id)
	if err != nil {
		return confirmationOutput{}, err
	}
	return confirmationOutput{Message: resp.Message}, nil
}
