-- name: GetGroupByID :one
SELECT
	*
FROM
	groups
WHERE
	id = $1
LIMIT
	1;

-- name: GetGroupByOrgAndName :one
SELECT
	*
FROM
	groups
WHERE
	organization_id = $1
AND
	name = $2
LIMIT
	1;

-- name: GetUserGroups :many
SELECT
	groups.*
FROM
	groups
JOIN
	group_members
ON
	groups.id = group_members.group_id
WHERE
	group_members.user_id = $1;


-- name: GetGroupMembers :many
SELECT
	users.*
FROM
	users
JOIN
	group_members
ON
	users.id = group_members.user_id
WHERE
	group_members.group_id = $1;

-- name: GetGroupsByOrganizationID :many
SELECT
	*
FROM
	groups
WHERE
	organization_id = $1;

-- name: InsertGroup :one
INSERT INTO groups (
	id,
	name,
	organization_id
)
VALUES
	( $1, $2, $3) RETURNING *;
