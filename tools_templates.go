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

const simpleDesignNote = "A simple design (editor simple, made in the dashboard's visual editor) can be " +
	"changed only in the dashboard: html and mjml are refused for it, while its name, subject, preview text, " +
	"sender and text can still change here."

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
			"template for send_template_email or to edit; get_template returns one template with its content. "+
			"The result holds one page and a next_cursor for the page after it.",
			listTemplates),
		newTool(ToolInfo{
			Name:           "get_template",
			Title:          "Get template",
			Toolset:        ToolsetTemplates,
			RequiredScopes: []string{"templates:read"},
			Annotations:    annRead(),
		}, "Returns one transactional template as sends use it (its published copy): its subject, sender, "+
			"content and the variables a send must fill. Use it before sending the template. "+variableKindsNote+
			" has_draft true means unpublished changes exist: get_template_draft returns them, and the content an "+
			"update_template call sends replaces the draft's content. "+simpleDesignNote,
			getTemplate),
		newTool(ToolInfo{
			Name:           "create_template",
			Title:          "Create template",
			Toolset:        ToolsetTemplates,
			RequiredScopes: []string{"templates:write"},
			Annotations:    annWrite(),
		}, "Creates a transactional template from HTML or MJML, with an optional plain text version. Use it to "+
			"build a template that send_template_email sends. The content goes to the template's draft, and "+
			"sends fail until it is published: set publish to true, or call publish_template later. Without "+
			"text, the text is made from the HTML. The result is the published copy, without its content: "+
			"unless publish was set it is still empty, and has_draft and get_template_draft show what was written.",
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
			"cannot change: give mjml for an advanced template and html for an html template. "+simpleDesignNote+
			" The result is the published copy, without its content: unless publish was set it does not show "+
			"the change, and has_draft and get_template_draft show what was written.",
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
			Name:           "get_template_draft",
			Title:          "Get template draft",
			Toolset:        ToolsetTemplates,
			RequiredScopes: []string{"templates:read"},
			Annotations:    annRead(),
		}, "Returns a transactional template's draft: the changes saved in the dashboard or through "+
			"update_template that are not published yet, with their content. Use it to review changes before "+
			"publishing them. A template without a draft returns a not-found error. "+variableKindsNote,
			getTemplateDraft),
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
			"content. Use it to compare an old version or before restoring it. "+variableKindsNote,
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
			"made from it only once it has HTML. "+simpleDesignNote,
			getCampaignTemplate),
		newTool(ToolInfo{
			Name:           "create_campaign_template",
			Title:          "Create campaign template",
			Toolset:        ToolsetTemplates,
			RequiredScopes: []string{"templates:write", "campaigns:read"},
			Annotations:    annWrite(),
		}, "Creates a campaign template, a reusable design for campaigns, from HTML or MJML with an optional "+
			"plain text version. Use it for a design several campaigns share; a campaign copies the design when "+
			"it is made, and later changes to the template do not reach it. It has no draft and no subject or "+
			"sender; the campaign owns those. The result is the campaign template, without its content.",
			createCampaignTemplate),
		newTool(ToolInfo{
			Name:           "update_campaign_template",
			Title:          "Update campaign template",
			Toolset:        ToolsetTemplates,
			RequiredScopes: []string{"templates:write", "campaigns:read"},
			Annotations:    annWrite(),
		}, "Changes a campaign template's name, content or both, at once. Use it to edit a shared campaign "+
			"design. Only the fields given change. "+
			"Campaigns already made from it keep their own copies. The editor cannot change: give mjml for an "+
			"advanced design and html for an html design. "+simpleDesignNote+" The result is the campaign "+
			"template, without its content.",
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

// templateContent is the content of a template, draft, version or campaign.
type templateContent struct {
	MJML         string `json:"mjml,omitempty" jsonschema:"MJML source of an advanced design."`
	HTML         string `json:"html,omitempty" jsonschema:"HTML of an html design, or the HTML made from a simple design. An advanced design returns mjml instead."`
	Text         string `json:"text,omitempty" jsonschema:"Plain text version."`
	TextIsCustom bool   `json:"text_is_custom" jsonschema:"True when the text was written rather than made from the HTML."`
}

func templateContentFrom(c *responses.TemplateContent) *templateContent {
	if c == nil {
		return nil
	}
	return &templateContent{MJML: c.MJML, HTML: c.HTML, Text: c.Text, TextIsCustom: c.TextIsCustom}
}

// contentInput is the content of a template or campaign write.
type contentInput struct {
	MJML string `json:"mjml,omitempty" jsonschema:"MJML source, for an advanced design. Give mjml or html, not both. Images and stylesheets must use https URLs."`
	HTML string `json:"html,omitempty" jsonschema:"HTML, for an html design. Give mjml or html, not both. Images and stylesheets must use https URLs."`
	Text string `json:"text,omitempty" jsonschema:"Plain text version. When omitted, it is made from the HTML."`
}

func (c contentInput) request() *requests.TemplateContentInput {
	if c.MJML == "" && c.HTML == "" && c.Text == "" {
		return nil
	}
	return &requests.TemplateContentInput{
		MJML: optionalString(c.MJML),
		HTML: optionalString(c.HTML),
		Text: optionalString(c.Text),
	}
}

// contentUpdateInput is the content of a template or campaign update.
type contentUpdateInput struct {
	MJML         string `json:"mjml,omitempty" jsonschema:"New MJML source, for an advanced design. Omit to keep the design."`
	HTML         string `json:"html,omitempty" jsonschema:"New HTML, for an html design. Omit to keep the design."`
	Text         string `json:"text,omitempty" jsonschema:"New plain text version. Omit to keep the text; beside new mjml or html, a text made from the old HTML is made again from the new one, and a written text is kept."`
	TextFromHTML bool   `json:"text_from_html,omitempty" jsonschema:"True to replace the text with one made from the HTML. Not with text."`
}

func (c contentUpdateInput) request() (*requests.TemplateContentInput, error) {
	if c.TextFromHTML && c.Text != "" {
		return nil, invalidInput("Give text or text_from_html, not both.")
	}
	in := contentInput{MJML: c.MJML, HTML: c.HTML, Text: c.Text}.request()
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
	Editor    string        `json:"editor" jsonschema:"advanced (MJML), html, or simple (the dashboard's visual editor, read-only here)."`
	Subject   string        `json:"subject,omitempty" jsonschema:"Published subject."`
	Preheader string        `json:"preheader,omitempty" jsonschema:"Published preview text."`
	From      *senderOutput `json:"from,omitempty" jsonschema:"Default sender; missing when every send must name one."`
	ReplyTo   string        `json:"reply_to,omitempty" jsonschema:"Default reply-to address."`
	HasDraft  bool          `json:"has_draft" jsonschema:"True when changes exist that are not published yet."`
	CreatedAt time.Time     `json:"created_at" jsonschema:"When the template was created."`
	UpdatedAt time.Time     `json:"updated_at" jsonschema:"When the published copy or the name last changed; draft edits do not change it."`
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

// templateResult is a template as a write returns it: with its variables, but
// without its content.
type templateResult struct {
	templateSummary
	Variables []templateVariable `json:"variables" jsonschema:"Variables of the published copy."`
}

func templateResultFrom(t *responses.Template) templateResult {
	return templateResult{templateSummary: templateSummaryFrom(t), Variables: templateVariablesFrom(t.Variables)}
}

// templateDetail is a template with its published content.
type templateDetail struct {
	templateResult
	Content *templateContent `json:"content,omitempty" jsonschema:"Published content; missing when the published copy has no body."`
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

type createTemplateInput struct {
	Name      string        `json:"name" jsonschema:"Template name."`
	Editor    string        `json:"editor,omitempty" jsonschema:"advanced, html or simple. Optional with mjml (advanced) or html (html); simple makes an empty visual design to finish in the dashboard."`
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

// templateDraftOutput is a template's unpublished draft.
type templateDraftOutput struct {
	TemplateID string             `json:"template_id" jsonschema:"Template ID."`
	UpdatedAt  time.Time          `json:"updated_at" jsonschema:"When the draft last changed."`
	Subject    string             `json:"subject,omitempty" jsonschema:"Draft subject."`
	Preheader  string             `json:"preheader,omitempty" jsonschema:"Draft preview text."`
	From       *senderOutput      `json:"from,omitempty" jsonschema:"Draft default sender."`
	ReplyTo    string             `json:"reply_to,omitempty" jsonschema:"Draft default reply-to address."`
	Variables  []templateVariable `json:"variables" jsonschema:"Variables the draft uses."`
	Content    *templateContent   `json:"content,omitempty" jsonschema:"Draft content; missing when the draft has no body."`
}

func getTemplateDraft(ctx context.Context, env *toolEnv, in templateIDInput) (templateDraftOutput, error) {
	id, err := parseID("template_id", in.TemplateID)
	if err != nil {
		return templateDraftOutput{}, err
	}
	d, _, err := env.client.TemplatesAPI.GetTemplateDraft(ctx, env.accountID, id)
	if err != nil {
		return templateDraftOutput{}, err
	}
	return templateDraftOutput{
		TemplateID: d.TemplateID.String(),
		UpdatedAt:  d.UpdatedAt,
		Subject:    d.Subject,
		Preheader:  d.Preheader,
		From:       senderFrom(d.From),
		ReplyTo:    replyToFrom(d.ReplyTo),
		Variables:  templateVariablesFrom(d.Variables),
		Content:    templateContentFrom(d.Content),
	}, nil
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
	Editor    string    `json:"editor" jsonschema:"advanced (MJML), html, or simple (the dashboard's visual editor, read-only here)."`
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
	Editor string `json:"editor,omitempty" jsonschema:"advanced, html or simple. Optional with mjml (advanced) or html (html); without content the template gets an empty design of this editor."`
	contentInput
}

func createCampaignTemplate(ctx context.Context, env *toolEnv, in createCampaignTemplateInput) (campaignTemplateSummary, error) {
	t, _, err := env.client.CampaignTemplatesAPI.CreateCampaignTemplate(ctx, env.accountID, requests.CreateCampaignTemplateRequest{
		Name:    in.Name,
		Editor:  optionalString(in.Editor),
		Content: in.request(),
	})
	if err != nil {
		return campaignTemplateSummary{}, err
	}
	return campaignTemplateSummaryFrom(t), nil
}

type updateCampaignTemplateInput struct {
	CampaignTemplateID string `json:"campaign_template_id" jsonschema:"Campaign template ID."`
	Name               string `json:"name,omitempty" jsonschema:"New name."`
	contentUpdateInput
}

func updateCampaignTemplate(ctx context.Context, env *toolEnv, in updateCampaignTemplateInput) (campaignTemplateSummary, error) {
	id, err := parseID("campaign_template_id", in.CampaignTemplateID)
	if err != nil {
		return campaignTemplateSummary{}, err
	}
	content, err := in.request()
	if err != nil {
		return campaignTemplateSummary{}, err
	}
	t, _, err := env.client.CampaignTemplatesAPI.UpdateCampaignTemplate(ctx, env.accountID, id, requests.UpdateCampaignTemplateRequest{
		Name:    optionalString(in.Name),
		Content: content,
	})
	if err != nil {
		return campaignTemplateSummary{}, err
	}
	return campaignTemplateSummaryFrom(t), nil
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
