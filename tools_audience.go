package ahasendmcp

import (
	"context"
	"strings"
	"time"

	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
)

func audienceTools() []tool {
	return []tool{
		newTool(ToolInfo{
			Name:           "list_contacts",
			Title:          "List contacts",
			Toolset:        ToolsetAudience,
			RequiredScopes: []string{"contacts:read"},
			Annotations:    annRead(),
		}, "Lists the contacts of this account, newest first, with their names, status, subscription state, "+
			"validation state and custom attributes. Use it to find contacts; filter by exact email, status, "+
			"subscription state or creation time. list_contact_lists shows a contact's lists. The result holds "+
			"one page and a next_cursor for the page after it.",
			listContacts),
		newTool(ToolInfo{
			Name:           "get_contact",
			Title:          "Get contact",
			Toolset:        ToolsetAudience,
			RequiredScopes: []string{"contacts:read"},
			Annotations:    annRead(),
		}, "Returns one contact, found by ID or email address, with its names, status, subscription state, "+
			"validation state and custom attributes. Use it to check a contact before changing it.",
			getContact),
		newTool(ToolInfo{
			Name:           "create_contact",
			Title:          "Create contact",
			Toolset:        ToolsetAudience,
			RequiredScopes: []string{"contacts:write"},
			Annotations:    annWrite(),
		}, "Creates one contact, on no list. Use it to add a single person; upsert_contacts adds or updates many "+
			"at once, and add_contacts_to_list or upsert_list_contact puts contacts on a list. Custom attributes "+
			"must match the attribute definitions made in the dashboard. An address that is already a contact "+
			"is refused. The result is the new contact.",
			createContact),
		newTool(ToolInfo{
			Name:           "upsert_contacts",
			Title:          "Create or update contacts",
			Toolset:        ToolsetAudience,
			RequiredScopes: []string{"contacts:write"},
			Annotations:    annWrite(),
		}, "Creates or updates up to 1,000 contacts in one call, matched by email address; it puts no contact on "+
			"a list. Use it to import contacts. For an existing contact, only the fields given change, and "+
			"attributes are merged, with a null value removing that attribute. The result counts the created, "+
			"updated and failed entries and gives each entry's outcome in input order, with the reason for "+
			"each failure.",
			upsertContacts),
		newTool(ToolInfo{
			Name:           "update_contact",
			Title:          "Update contact",
			Toolset:        ToolsetAudience,
			RequiredScopes: []string{"contacts:write"},
			Annotations:    annWrite(),
		}, "Changes one contact, found by ID or email address. Use it to correct a contact's details or "+
			"status. Only the fields given change; attributes are "+
			"merged, with a null value removing that attribute. Setting unsubscribed to false subscribes the "+
			"contact again, which is right only when the contact asked for it. The result is the changed contact.",
			updateContact),
		newTool(ToolInfo{
			Name:           "delete_contact",
			Title:          "Delete contact",
			Toolset:        ToolsetAudience,
			RequiredScopes: []string{"contacts:delete"},
			Annotations:    annDestructive(),
		}, "Deletes one contact, found by ID or email address, for good, with all its list memberships, "+
			"including unsubscribed and complained ones, so an address added again is not known to have "+
			"unsubscribed. Use it only when the user asks to remove the contact; to stop campaigns to it, "+
			"unsubscribe it with update_contact instead. The result is AhaSend's confirmation.",
			deleteContact),
		newTool(ToolInfo{
			Name:           "list_contact_lists",
			Title:          "List a contact's lists",
			Toolset:        ToolsetAudience,
			RequiredScopes: []string{"lists:read"},
			Annotations:    annRead(),
		}, "Lists the lists one contact, found by ID or email address, is on, with the subscription status of "+
			"each membership. Use it to see where a contact gets campaigns from. The result holds one page and a "+
			"next_cursor for the page after it.",
			listContactLists),
		newTool(ToolInfo{
			Name:           "list_lists",
			Title:          "List lists",
			Toolset:        ToolsetAudience,
			RequiredScopes: []string{"lists:read"},
			Annotations:    annRead(),
		}, "Lists the contact lists of this account, newest first, with each list's contact_count: the "+
			"confirmed members a campaign to it would reach, counting only enabled contacts that have not "+
			"unsubscribed from every list and were not found invalid. Use it to pick lists for a campaign; name matches any list "+
			"whose name contains the text, ignoring case. The result holds one page and a next_cursor for the "+
			"page after it.",
			listLists),
		newTool(ToolInfo{
			Name:           "get_list",
			Title:          "Get list",
			Toolset:        ToolsetAudience,
			RequiredScopes: []string{"lists:read"},
			Annotations:    annRead(),
		}, "Returns one contact list with its description, tags and contact_count: the confirmed members a "+
			"campaign to it would reach, counting only enabled contacts that have not unsubscribed from every "+
			"list and were not found invalid. Use it to check a list before a campaign is sent to it.",
			getList),
		newTool(ToolInfo{
			Name:           "create_list",
			Title:          "Create list",
			Toolset:        ToolsetAudience,
			RequiredScopes: []string{"lists:write"},
			Annotations:    annWrite(),
		}, "Creates an empty contact list. Use it before adding contacts for a campaign. The result is the new "+
			"list with its ID.",
			createList),
		newTool(ToolInfo{
			Name:           "update_list",
			Title:          "Update list",
			Toolset:        ToolsetAudience,
			RequiredScopes: []string{"lists:write"},
			Annotations:    annWrite(),
		}, "Changes a contact list's name, description or tags. Use it when the user wants the list renamed "+
			"or described differently. Only the fields given change; tags replace the "+
			"list's tags, and an empty tags array removes them. The result is the changed list.",
			updateList),
		newTool(ToolInfo{
			Name:           "delete_list",
			Title:          "Delete list",
			Toolset:        ToolsetAudience,
			RequiredScopes: []string{"lists:delete"},
			Annotations:    annDestructive(),
		}, "Deletes a contact list; its contacts stay in the account. A list that a campaign still needs cannot "+
			"be deleted. Use it only when the user asks to remove the list. The result is AhaSend's confirmation.",
			deleteList),
		newTool(ToolInfo{
			Name:           "list_list_contacts",
			Title:          "List a list's contacts",
			Toolset:        ToolsetAudience,
			RequiredScopes: []string{"lists:read"},
			Annotations:    annRead(),
		}, "Lists the memberships of one contact list, newest first: each contact's ID, email address and "+
			"subscription status on the list. Use it to see who is on a list; filter by subscription status or "+
			"exact email. get_contact returns a contact's other fields. The result holds one page and a "+
			"next_cursor for the page after it.",
			listListContacts),
		newTool(ToolInfo{
			Name:           "add_contacts_to_list",
			Title:          "Add contacts to list",
			Toolset:        ToolsetAudience,
			RequiredScopes: []string{"lists:write"},
			Annotations:    annWrite(),
		}, "Adds up to 1,000 existing contacts, by ID or email address, to one list as confirmed members. "+
			"Use it to fill a list for a campaign. "+
			"Contacts must exist first; create them with create_contact or upsert_contacts. A contact already on "+
			"the list is left as it is: its entry is already_member, and its subscription_status shows whether "+
			"it will get campaigns. The result counts the added, skipped and failed entries and gives each "+
			"entry's outcome in input order, with the reason for each failure.",
			addContactsToList),
		newTool(ToolInfo{
			Name:           "upsert_list_contact",
			Title:          "Add or update list contact",
			Toolset:        ToolsetAudience,
			RequiredScopes: []string{"lists:write"},
			Annotations:    annWrite(),
		}, "Adds one existing contact, by ID or email address, to a list, or changes its subscription status "+
			"on that list. Use it for one contact; add_contacts_to_list adds many. Without subscription_status a new member is confirmed and another member's status "+
			"is left as it is. Use unsubscribed to stop campaigns from this list while keeping the record. A "+
			"complained membership (the contact reported spam) cannot be changed: every call for it is refused. "+
			"The result is the membership.",
			upsertListContact),
		newTool(ToolInfo{
			Name:           "remove_contact_from_list",
			Title:          "Remove contact from list",
			Toolset:        ToolsetAudience,
			RequiredScopes: []string{"lists:write"},
			Annotations:    annDestructive(),
		}, "Removes one contact, by ID or email address, from a list, and forgets any unsubscribe that "+
			"membership recorded. To stop campaigns from the list but keep the unsubscribe, use "+
			"upsert_list_contact with unsubscribed instead. A complained membership cannot be removed. The "+
			"result is AhaSend's confirmation.",
			removeContactFromList),
	}
}

// contactRef reads a contact ID or email address. Stored addresses are
// lowercase, so an address is lowercased.
func contactRef(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "", invalidInput("contact is required: give the contact's ID or email address.")
	}
	return value, nil
}

// contactOutput is one contact.
type contactOutput struct {
	ID               string         `json:"id" jsonschema:"Contact ID."`
	Email            string         `json:"email" jsonschema:"Email address."`
	FirstName        string         `json:"first_name,omitempty" jsonschema:"First name."`
	LastName         string         `json:"last_name,omitempty" jsonschema:"Last name."`
	Status           string         `json:"status" jsonschema:"enabled, disabled or blocked; only enabled contacts get campaigns."`
	StatusReason     string         `json:"status_reason,omitempty" jsonschema:"Why the status was set."`
	Unsubscribed     bool           `json:"unsubscribed" jsonschema:"True when the contact unsubscribed from every list."`
	UnsubscribedAt   *time.Time     `json:"unsubscribed_at,omitempty" jsonschema:"When the contact unsubscribed."`
	ValidationStatus string         `json:"validation_status" jsonschema:"unvalidated, valid, invalid, risky or unknown."`
	LastValidatedAt  *time.Time     `json:"last_validated_at,omitempty" jsonschema:"When the address was last validated."`
	Attributes       map[string]any `json:"attributes,omitempty" jsonschema:"Custom attributes by name."`
	CreatedAt        time.Time      `json:"created_at" jsonschema:"When the contact was created."`
	UpdatedAt        time.Time      `json:"updated_at" jsonschema:"When the contact last changed."`
}

func contactFrom(c *responses.Contact) contactOutput {
	return contactOutput{
		ID:               c.ID.String(),
		Email:            c.Email,
		FirstName:        c.FirstName,
		LastName:         c.LastName,
		Status:           c.Status,
		StatusReason:     c.StatusReason,
		Unsubscribed:     c.Unsubscribed,
		UnsubscribedAt:   c.UnsubscribedAt,
		ValidationStatus: c.ValidationStatus,
		LastValidatedAt:  c.LastValidatedAt,
		Attributes:       c.Attributes,
		CreatedAt:        c.CreatedAt,
		UpdatedAt:        c.UpdatedAt,
	}
}

type listContactsInput struct {
	pageInput
	Email      string     `json:"email,omitempty" jsonschema:"Exact email address."`
	Status     string     `json:"status,omitempty" jsonschema:"enabled, disabled or blocked."`
	Subscribed *bool      `json:"subscribed,omitempty" jsonschema:"true for contacts not unsubscribed from every list, false for unsubscribed ones."`
	FromTime   *time.Time `json:"from_time,omitempty" jsonschema:"RFC 3339 time; only contacts created at or after it."`
	ToTime     *time.Time `json:"to_time,omitempty" jsonschema:"RFC 3339 time; only contacts created at or before it."`
}

type listContactsOutput struct {
	Contacts []contactOutput `json:"contacts" jsonschema:"This page's contacts."`
	pageOutput
}

func listContacts(ctx context.Context, env *toolEnv, in listContactsInput) (listContactsOutput, error) {
	page, err := in.params()
	if err != nil {
		return listContactsOutput{}, err
	}
	resp, _, err := env.client.ContactsAPI.GetContacts(ctx, env.accountID, requests.GetContactsParams{
		Limit:      page.Limit,
		After:      page.After,
		Email:      optionalString(strings.ToLower(strings.TrimSpace(in.Email))),
		Status:     optionalString(in.Status),
		Subscribed: in.Subscribed,
		FromTime:   in.FromTime,
		ToTime:     in.ToTime,
	})
	if err != nil {
		return listContactsOutput{}, err
	}
	out := listContactsOutput{Contacts: []contactOutput{}, pageOutput: contactPageOutputFrom(resp.Pagination)}
	for i := range resp.Data {
		out.Contacts = append(out.Contacts, contactFrom(&resp.Data[i]))
	}
	return out, nil
}

type contactRefInput struct {
	Contact string `json:"contact" jsonschema:"The contact's ID or email address."`
}

func getContact(ctx context.Context, env *toolEnv, in contactRefInput) (contactOutput, error) {
	ref, err := contactRef(in.Contact)
	if err != nil {
		return contactOutput{}, err
	}
	c, _, err := env.client.ContactsAPI.GetContact(ctx, env.accountID, ref)
	if err != nil {
		return contactOutput{}, err
	}
	return contactFrom(c), nil
}

// contactFields are the contact fields the write tools take.
type contactFields struct {
	FirstName    string         `json:"first_name,omitempty" jsonschema:"First name."`
	LastName     string         `json:"last_name,omitempty" jsonschema:"Last name."`
	Status       string         `json:"status,omitempty" jsonschema:"enabled, disabled or blocked. New contacts are enabled."`
	StatusReason string         `json:"status_reason,omitempty" jsonschema:"Why the status was set."`
	Unsubscribed *bool          `json:"unsubscribed,omitempty" jsonschema:"true records the contact as unsubscribed from every list. For a contact that already exists (upsert_contacts), false subscribes it again."`
	Attributes   map[string]any `json:"attributes,omitempty" jsonschema:"Custom attributes by name, as defined in the dashboard."`
}

type createContactInput struct {
	Email string `json:"email" jsonschema:"Email address."`
	contactFields
}

func createContact(ctx context.Context, env *toolEnv, in createContactInput) (contactOutput, error) {
	c, _, err := env.client.ContactsAPI.CreateContact(ctx, env.accountID, requests.CreateContactRequest{
		Email:        strings.ToLower(strings.TrimSpace(in.Email)),
		FirstName:    optionalString(in.FirstName),
		LastName:     optionalString(in.LastName),
		Status:       optionalString(in.Status),
		StatusReason: optionalString(in.StatusReason),
		Unsubscribed: in.Unsubscribed,
		Attributes:   in.Attributes,
	})
	if err != nil {
		return contactOutput{}, err
	}
	return contactFrom(c), nil
}

type upsertContactsInput struct {
	Contacts []createContactInput `json:"contacts" jsonschema:"Contacts to create or update, 1 to 1,000, matched by email address."`
}

type contactBatchResult struct {
	Position  int    `json:"position" jsonschema:"Index of the entry in contacts, from 0."`
	Email     string `json:"email" jsonschema:"The entry's email address."`
	Outcome   string `json:"outcome" jsonschema:"created, updated or failed."`
	ContactID string `json:"contact_id,omitempty" jsonschema:"The contact's ID, unless the entry failed."`
	Reason    string `json:"reason,omitempty" jsonschema:"Why the entry failed."`
}

type upsertContactsOutput struct {
	Created int                  `json:"created" jsonschema:"Contacts created."`
	Updated int                  `json:"updated" jsonschema:"Contacts updated."`
	Failed  int                  `json:"failed" jsonschema:"Entries that failed."`
	Results []contactBatchResult `json:"results" jsonschema:"Each entry's outcome, in input order."`
}

func upsertContacts(ctx context.Context, env *toolEnv, in upsertContactsInput) (upsertContactsOutput, error) {
	req := requests.BatchUpsertContactsRequest{Data: make([]requests.BatchUpsertContactInput, 0, len(in.Contacts))}
	for _, c := range in.Contacts {
		req.Data = append(req.Data, requests.BatchUpsertContactInput{
			Email:        strings.ToLower(strings.TrimSpace(c.Email)),
			FirstName:    optionalString(c.FirstName),
			LastName:     optionalString(c.LastName),
			Status:       optionalString(c.Status),
			StatusReason: optionalString(c.StatusReason),
			Unsubscribed: c.Unsubscribed,
			Attributes:   c.Attributes,
		})
	}
	resp, _, err := env.client.ContactsAPI.BatchUpsertContacts(ctx, env.accountID, req)
	if err != nil {
		return upsertContactsOutput{}, err
	}
	out := upsertContactsOutput{
		Created: resp.Created,
		Updated: resp.Updated,
		Failed:  resp.Failed,
		Results: make([]contactBatchResult, 0, len(resp.Data)),
	}
	for _, r := range resp.Data {
		result := contactBatchResult{Position: r.Position, Email: r.Email, Outcome: r.Outcome, Reason: r.Reason}
		if r.Contact != nil {
			result.ContactID = r.Contact.ID.String()
		}
		out.Results = append(out.Results, result)
	}
	return out, nil
}

type updateContactInput struct {
	Contact      string         `json:"contact" jsonschema:"The contact's ID or current email address."`
	Email        string         `json:"email,omitempty" jsonschema:"New email address."`
	FirstName    *string        `json:"first_name,omitempty" jsonschema:"New first name. An empty value clears it."`
	LastName     *string        `json:"last_name,omitempty" jsonschema:"New last name. An empty value clears it."`
	Status       string         `json:"status,omitempty" jsonschema:"enabled, disabled or blocked."`
	StatusReason *string        `json:"status_reason,omitempty" jsonschema:"Why the status was set. An empty value clears it."`
	Unsubscribed *bool          `json:"unsubscribed,omitempty" jsonschema:"true to unsubscribe the contact from every list; false to subscribe it again."`
	Attributes   map[string]any `json:"attributes,omitempty" jsonschema:"Custom attributes to set, by name, as defined in the dashboard; a null value removes that attribute."`
}

func updateContact(ctx context.Context, env *toolEnv, in updateContactInput) (contactOutput, error) {
	ref, err := contactRef(in.Contact)
	if err != nil {
		return contactOutput{}, err
	}
	c, _, err := env.client.ContactsAPI.UpdateContact(ctx, env.accountID, ref, requests.UpdateContactRequest{
		Email:        optionalString(strings.ToLower(strings.TrimSpace(in.Email))),
		FirstName:    in.FirstName,
		LastName:     in.LastName,
		Status:       optionalString(in.Status),
		StatusReason: in.StatusReason,
		Unsubscribed: in.Unsubscribed,
		Attributes:   in.Attributes,
	})
	if err != nil {
		return contactOutput{}, err
	}
	return contactFrom(c), nil
}

func deleteContact(ctx context.Context, env *toolEnv, in contactRefInput) (confirmationOutput, error) {
	ref, err := contactRef(in.Contact)
	if err != nil {
		return confirmationOutput{}, err
	}
	resp, _, err := env.client.ContactsAPI.DeleteContact(ctx, env.accountID, ref)
	if err != nil {
		return confirmationOutput{}, err
	}
	return confirmationOutput{Message: resp.Message}, nil
}

// membershipOutput is one contact's membership of one list.
type membershipOutput struct {
	ListID             string     `json:"list_id" jsonschema:"List ID."`
	ListName           string     `json:"list_name,omitempty" jsonschema:"List name."`
	ContactID          string     `json:"contact_id" jsonschema:"Contact ID."`
	Email              string     `json:"email" jsonschema:"The contact's email address."`
	SubscriptionStatus string     `json:"subscription_status" jsonschema:"confirmed, unconfirmed, unsubscribed or complained (the recipient reported spam). Only confirmed members get campaigns."`
	SubscribedAt       *time.Time `json:"subscribed_at,omitempty" jsonschema:"When the contact subscribed."`
	UnsubscribedAt     *time.Time `json:"unsubscribed_at,omitempty" jsonschema:"When the contact unsubscribed."`
	CreatedAt          time.Time  `json:"created_at" jsonschema:"When the contact was added to the list."`
}

func membershipFrom(m *responses.ListContact) membershipOutput {
	out := membershipOutput{
		ListID:             m.ListID.String(),
		ContactID:          m.ContactID.String(),
		Email:              m.Email,
		SubscriptionStatus: m.SubscriptionStatus,
		SubscribedAt:       m.SubscribedAt,
		UnsubscribedAt:     m.UnsubscribedAt,
		CreatedAt:          m.CreatedAt,
	}
	if m.List != nil {
		out.ListName = m.List.Name
	}
	return out
}

type listContactListsInput struct {
	pageInput
	Contact            string `json:"contact" jsonschema:"The contact's ID or email address."`
	SubscriptionStatus string `json:"subscription_status,omitempty" jsonschema:"Only memberships with this status: confirmed, unconfirmed, unsubscribed or complained."`
}

type membershipsOutput struct {
	Memberships []membershipOutput `json:"memberships" jsonschema:"This page's memberships."`
	pageOutput
}

func membershipsFrom(resp *responses.PaginatedListContactsResponse) membershipsOutput {
	out := membershipsOutput{Memberships: []membershipOutput{}, pageOutput: contactPageOutputFrom(resp.Pagination)}
	for i := range resp.Data {
		out.Memberships = append(out.Memberships, membershipFrom(&resp.Data[i]))
	}
	return out
}

func listContactLists(ctx context.Context, env *toolEnv, in listContactListsInput) (membershipsOutput, error) {
	ref, err := contactRef(in.Contact)
	if err != nil {
		return membershipsOutput{}, err
	}
	page, err := in.params()
	if err != nil {
		return membershipsOutput{}, err
	}
	resp, _, err := env.client.ListsAPI.GetContactLists(ctx, env.accountID, ref, requests.GetContactListsParams{
		Limit:              page.Limit,
		After:              page.After,
		SubscriptionStatus: optionalString(in.SubscriptionStatus),
	})
	if err != nil {
		return membershipsOutput{}, err
	}
	return membershipsFrom(resp), nil
}

// listOutput is one contact list.
type listOutput struct {
	ID           string    `json:"id" jsonschema:"List ID."`
	Name         string    `json:"name" jsonschema:"List name."`
	Description  string    `json:"description,omitempty" jsonschema:"Description."`
	Tags         []string  `json:"tags,omitempty" jsonschema:"Tags."`
	ContactCount int       `json:"contact_count" jsonschema:"Confirmed members a campaign to this list would reach: enabled contacts not unsubscribed from every list and not found invalid."`
	CreatedAt    time.Time `json:"created_at" jsonschema:"When the list was created."`
	UpdatedAt    time.Time `json:"updated_at" jsonschema:"When the list last changed."`
}

func listFrom(l *responses.ContactList) listOutput {
	return listOutput{
		ID:           l.ID.String(),
		Name:         l.Name,
		Description:  l.Description,
		Tags:         l.Tags,
		ContactCount: l.ContactCount,
		CreatedAt:    l.CreatedAt,
		UpdatedAt:    l.UpdatedAt,
	}
}

type listListsInput struct {
	pageInput
	Name string `json:"name,omitempty" jsonschema:"Text in the list name; case is ignored."`
}

type listListsOutput struct {
	Lists []listOutput `json:"lists" jsonschema:"This page's lists."`
	pageOutput
}

func listLists(ctx context.Context, env *toolEnv, in listListsInput) (listListsOutput, error) {
	page, err := in.params()
	if err != nil {
		return listListsOutput{}, err
	}
	resp, _, err := env.client.ListsAPI.GetLists(ctx, env.accountID, requests.GetListsParams{
		Limit: page.Limit,
		After: page.After,
		Name:  optionalString(strings.TrimSpace(in.Name)),
	})
	if err != nil {
		return listListsOutput{}, err
	}
	out := listListsOutput{Lists: []listOutput{}, pageOutput: contactPageOutputFrom(resp.Pagination)}
	for i := range resp.Data {
		out.Lists = append(out.Lists, listFrom(&resp.Data[i]))
	}
	return out, nil
}

type listIDInput struct {
	ListID string `json:"list_id" jsonschema:"List ID."`
}

func getList(ctx context.Context, env *toolEnv, in listIDInput) (listOutput, error) {
	id, err := parseID("list_id", in.ListID)
	if err != nil {
		return listOutput{}, err
	}
	l, _, err := env.client.ListsAPI.GetList(ctx, env.accountID, id)
	if err != nil {
		return listOutput{}, err
	}
	return listFrom(l), nil
}

type createListInput struct {
	Name        string   `json:"name" jsonschema:"List name."`
	Description string   `json:"description,omitempty" jsonschema:"Description."`
	Tags        []string `json:"tags,omitempty" jsonschema:"Tags."`
}

func createList(ctx context.Context, env *toolEnv, in createListInput) (listOutput, error) {
	l, _, err := env.client.ListsAPI.CreateList(ctx, env.accountID, requests.CreateListRequest{
		Name:        in.Name,
		Description: optionalString(in.Description),
		Tags:        in.Tags,
	})
	if err != nil {
		return listOutput{}, err
	}
	return listFrom(l), nil
}

type updateListInput struct {
	ListID      string   `json:"list_id" jsonschema:"List ID."`
	Name        string   `json:"name,omitempty" jsonschema:"New name."`
	Description *string  `json:"description,omitempty" jsonschema:"New description. An empty value clears it."`
	Tags        []string `json:"tags,omitempty" jsonschema:"New tags, replacing the list's tags; an empty array removes them."`
}

func updateList(ctx context.Context, env *toolEnv, in updateListInput) (listOutput, error) {
	id, err := parseID("list_id", in.ListID)
	if err != nil {
		return listOutput{}, err
	}
	req := requests.UpdateListRequest{
		Name:        optionalString(in.Name),
		Description: in.Description,
	}
	if in.Tags != nil {
		req.Tags = &in.Tags
	}
	l, _, err := env.client.ListsAPI.UpdateList(ctx, env.accountID, id, req)
	if err != nil {
		return listOutput{}, err
	}
	return listFrom(l), nil
}

func deleteList(ctx context.Context, env *toolEnv, in listIDInput) (confirmationOutput, error) {
	id, err := parseID("list_id", in.ListID)
	if err != nil {
		return confirmationOutput{}, err
	}
	resp, _, err := env.client.ListsAPI.DeleteList(ctx, env.accountID, id)
	if err != nil {
		return confirmationOutput{}, err
	}
	return confirmationOutput{Message: resp.Message}, nil
}

type listListContactsInput struct {
	pageInput
	ListID             string `json:"list_id" jsonschema:"List ID."`
	SubscriptionStatus string `json:"subscription_status,omitempty" jsonschema:"Only members with this status: confirmed, unconfirmed, unsubscribed or complained."`
	Email              string `json:"email,omitempty" jsonschema:"Exact email address."`
}

func listListContacts(ctx context.Context, env *toolEnv, in listListContactsInput) (membershipsOutput, error) {
	id, err := parseID("list_id", in.ListID)
	if err != nil {
		return membershipsOutput{}, err
	}
	page, err := in.params()
	if err != nil {
		return membershipsOutput{}, err
	}
	resp, _, err := env.client.ListsAPI.GetListContacts(ctx, env.accountID, id, requests.GetListContactsParams{
		Limit:              page.Limit,
		After:              page.After,
		SubscriptionStatus: optionalString(in.SubscriptionStatus),
		Email:              optionalString(strings.ToLower(strings.TrimSpace(in.Email))),
	})
	if err != nil {
		return membershipsOutput{}, err
	}
	return membershipsFrom(resp), nil
}

type addContactsToListInput struct {
	ListID   string   `json:"list_id" jsonschema:"List ID."`
	Contacts []string `json:"contacts" jsonschema:"1 to 1,000 contacts, each by ID or email address."`
}

type listBatchResult struct {
	Position           int    `json:"position" jsonschema:"Index of the entry in contacts, from 0."`
	Contact            string `json:"contact,omitempty" jsonschema:"The ID or email address the entry gave."`
	Outcome            string `json:"outcome" jsonschema:"added, already_member, not_found or invalid."`
	ContactID          string `json:"contact_id,omitempty" jsonschema:"The contact's ID, when the contact is on the list."`
	SubscriptionStatus string `json:"subscription_status,omitempty" jsonschema:"The membership's status on the list, when the contact is on it; only confirmed members get campaigns."`
	Reason             string `json:"reason,omitempty" jsonschema:"Why the entry failed."`
}

type addContactsToListOutput struct {
	Added   int               `json:"added" jsonschema:"Contacts added."`
	Skipped int               `json:"skipped" jsonschema:"Contacts already on the list."`
	Failed  int               `json:"failed" jsonschema:"Entries that failed."`
	Results []listBatchResult `json:"results" jsonschema:"Each entry's outcome, in input order."`
}

func addContactsToList(ctx context.Context, env *toolEnv, in addContactsToListInput) (addContactsToListOutput, error) {
	id, err := parseID("list_id", in.ListID)
	if err != nil {
		return addContactsToListOutput{}, err
	}
	req := requests.BatchAddListContactsRequest{Data: make([]requests.BatchAddListContactInput, 0, len(in.Contacts))}
	for i, c := range in.Contacts {
		c = strings.TrimSpace(c)
		if strings.Contains(c, "@") {
			req.Data = append(req.Data, requests.BatchAddListContactInput{Email: ptr(strings.ToLower(c))})
			continue
		}
		// Without an @ the entry can only be an ID.
		contactID, err := uuid.Parse(c)
		if err != nil {
			return addContactsToListOutput{}, invalidInput("contacts[%d] (%q) is a malformed contact ID, and not an email address.", i, c)
		}
		req.Data = append(req.Data, requests.BatchAddListContactInput{ID: &contactID})
	}
	resp, _, err := env.client.ListsAPI.BatchAddListContacts(ctx, env.accountID, id, req)
	if err != nil {
		return addContactsToListOutput{}, err
	}
	out := addContactsToListOutput{
		Added:   resp.Added,
		Skipped: resp.Skipped,
		Failed:  resp.Failed,
		Results: make([]listBatchResult, 0, len(resp.Data)),
	}
	for _, r := range resp.Data {
		result := listBatchResult{Position: r.Position, Contact: r.Email, Outcome: r.Outcome, Reason: r.Reason}
		if r.ID != nil {
			result.Contact = r.ID.String()
		}
		if r.Membership != nil {
			result.ContactID = r.Membership.ContactID.String()
			result.SubscriptionStatus = r.Membership.SubscriptionStatus
		}
		out.Results = append(out.Results, result)
	}
	return out, nil
}

type listContactInput struct {
	ListID  string `json:"list_id" jsonschema:"List ID."`
	Contact string `json:"contact" jsonschema:"The contact's ID or email address."`
}

func (in listContactInput) refs() (uuid.UUID, string, error) {
	id, err := parseID("list_id", in.ListID)
	if err != nil {
		return uuid.Nil, "", err
	}
	ref, err := contactRef(in.Contact)
	return id, ref, err
}

type upsertListContactInput struct {
	listContactInput
	SubscriptionStatus string `json:"subscription_status,omitempty" jsonschema:"confirmed, unconfirmed or unsubscribed. Omit to confirm a new member and leave another member's status as it is."`
}

func upsertListContact(ctx context.Context, env *toolEnv, in upsertListContactInput) (membershipOutput, error) {
	listID, ref, err := in.refs()
	if err != nil {
		return membershipOutput{}, err
	}
	m, _, err := env.client.ListsAPI.UpsertListContact(ctx, env.accountID, listID, ref, requests.UpsertListContactRequest{
		SubscriptionStatus: optionalString(in.SubscriptionStatus),
	})
	if err != nil {
		return membershipOutput{}, err
	}
	return membershipFrom(m), nil
}

func removeContactFromList(ctx context.Context, env *toolEnv, in listContactInput) (confirmationOutput, error) {
	listID, ref, err := in.refs()
	if err != nil {
		return confirmationOutput{}, err
	}
	resp, _, err := env.client.ListsAPI.DeleteListContact(ctx, env.accountID, listID, ref)
	if err != nil {
		return confirmationOutput{}, err
	}
	return confirmationOutput{Message: resp.Message}, nil
}
