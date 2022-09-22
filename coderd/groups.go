package coderd

import (
	"database/sql"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"golang.org/x/xerrors"

	"github.com/coder/coder/coderd/database"
	"github.com/coder/coder/coderd/httpapi"
	"github.com/coder/coder/coderd/httpmw"
	"github.com/coder/coder/codersdk"
)

func (api *API) postGroupByOrganization(rw http.ResponseWriter, r *http.Request) {
	var (
		ctx = r.Context()
		org = httpmw.OrganizationParam(r)
	)

	// if api.Authorize(r, rbac.ActionCreate, rbac.ResourceGroup.InOrg(org.ID)) {
	// 	http.NotFound(rw, r)
	// 	return
	// }

	var req codersdk.CreateGroupRequest
	if !httpapi.Read(rw, r, &req) {
		return
	}

	group, err := api.Database.InsertGroup(ctx, database.InsertGroupParams{
		ID:             uuid.New(),
		Name:           req.Name,
		OrganizationID: org.ID,
	})
	if database.IsUniqueViolation(err) {
		httpapi.Write(rw, http.StatusConflict, codersdk.Response{
			Message: fmt.Sprintf("Group with name %q already exists.", req.Name),
		})
		return
	}
	if err != nil {
		httpapi.InternalServerError(rw, err)
		return
	}

	httpapi.Write(rw, http.StatusCreated, group)
}

func (api *API) patchGroup(rw http.ResponseWriter, r *http.Request) {
	var (
		ctx   = r.Context()
		group = httpmw.GroupParam(r)
	)

	// if api.Authorize(r, rbac.ActionUpdate, rbac.ResourceGroup.InOrg(group.OrganizationID)) {
	// 	http.NotFound(rw, r)
	// 	return
	// }

	var req codersdk.PatchGroupRequest
	if !httpapi.Read(rw, r, &req) {
		return
	}

	users := make([]string, 0, len(req.AddUsers)+len(req.RemoveUsers))
	users = append(users, req.AddUsers...)
	users = append(users, req.RemoveUsers...)

	for _, id := range users {
		if _, err := uuid.Parse(id); err != nil {
			httpapi.Write(rw, http.StatusBadRequest, codersdk.Response{
				Message: fmt.Sprintf("ID %q must be a valid user UUID.", id),
			})
			return
		}
		// TODO: It would be nice to enforce this at the schema level
		// but unfortunately our org_members table does not have an ID.
		_, err := api.Database.GetOrganizationMemberByUserID(ctx, database.GetOrganizationMemberByUserIDParams{
			OrganizationID: group.OrganizationID,
			UserID:         uuid.MustParse(id),
		})
		if xerrors.Is(err, sql.ErrNoRows) {
			httpapi.Write(rw, http.StatusPreconditionFailed, codersdk.Response{
				Message: fmt.Sprintf("User %q must be a member of organization %q", id, group.ID),
			})
			return
		}
		if err != nil {
			httpapi.InternalServerError(rw, err)
			return
		}
	}
	if req.Name != "" {
		_, err := api.Database.GetGroupByOrgAndName(ctx, database.GetGroupByOrgAndNameParams{
			OrganizationID: group.OrganizationID,
			Name:           req.Name,
		})
		if err == nil {
			httpapi.Write(rw, http.StatusConflict, codersdk.Response{
				Message: fmt.Sprintf("A group with name %q already exists.", req.Name),
			})
			return
		}
	}

	err := api.Database.InTx(func(tx database.Store) error {
		if req.Name != "" {
			var err error
			group, err = tx.UpdateGroupByID(ctx, database.UpdateGroupByIDParams{
				ID:   group.ID,
				Name: req.Name,
			})
			if err != nil {
				return xerrors.Errorf("update group by ID: %w", err)
			}
		}
		for _, id := range req.AddUsers {
			err := tx.InsertGroupMember(ctx, database.InsertGroupMemberParams{
				GroupID: group.ID,
				UserID:  uuid.MustParse(id),
			})
			if err != nil {
				return xerrors.Errorf("insert group member %q: %w", id, err)
			}
		}
		for _, id := range req.RemoveUsers {
			err := tx.DeleteGroupMember(ctx, uuid.MustParse(id))
			if err != nil {
				return xerrors.Errorf("insert group member %q: %w", id, err)
			}
		}
		return nil
	})
	if xerrors.Is(err, sql.ErrNoRows) {
		httpapi.Write(rw, http.StatusPreconditionFailed, codersdk.Response{
			Message: "Failed to add or remove non-existent group member",
			Detail:  err.Error(),
		})
		return
	}

	members, err := api.Database.GetGroupMembers(ctx, group.ID)
	if err != nil {
		httpapi.InternalServerError(rw, err)
		return
	}

	httpapi.Write(rw, http.StatusOK, convertGroup(group, members))
}

func (api *API) deleteGroup(rw http.ResponseWriter, r *http.Request) {

}

func (api *API) group(rw http.ResponseWriter, r *http.Request) {

}

func (api *API) groups(rw http.ResponseWriter, r *http.Request) {

}

func convertGroup(g database.Group, users []database.User) codersdk.Group {
	// It's ridiculous to query all the orgs of a user here
	// especially since as of the writing of this comment there
	// is only one org. So we pretend everyone is only part of
	// the group's organization.
	orgs := make(map[uuid.UUID][]uuid.UUID)
	for _, user := range users {
		orgs[user.ID] = []uuid.UUID{g.OrganizationID}
	}
	return codersdk.Group{
		ID:             g.ID,
		Name:           g.Name,
		OrganizationID: g.OrganizationID,
		Members:        convertUsers(users, orgs),
	}
}
