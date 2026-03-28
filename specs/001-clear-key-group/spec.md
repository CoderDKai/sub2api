# Feature Specification: Clear Key Group Bindings

**Feature Branch**: `001-clear-key-group`  
**Created**: 2026-01-20  
**Status**: Draft  
**Input**: User description: "When an admin revokes a user's access to a group, existing keys previously bound to that group still work. The system should clear group bindings during the update (set the key to have no group) so revoked access stops, while key deletion still works."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Revoke Group Access Clears Key Binding (Priority: P1)

When an admin removes a group from a user's allowed groups, any of that user's keys that are bound to the removed group are automatically unassigned so the key can no longer be used for that group.

**Why this priority**: Preventing continued access after revocation is critical for permission correctness and security.

**Independent Test**: Remove a group from a user who has a key bound to that group and verify the key becomes unassigned and cannot access that group.

**Acceptance Scenarios**:

1. **Given** a user allowed to access Group A with a key bound to Group A, **When** an admin removes Group A from the user's allowed groups, **Then** within 60 seconds the key no longer has a Group A assignment and access to Group A is blocked.
2. **Given** a user with multiple keys across multiple groups, **When** an admin removes Group A, **Then** only keys bound to Group A are unassigned and keys bound to remaining groups are unchanged.

---

### User Story 2 - Manage Keys After Unassignment (Priority: P2)

After group access is revoked, keys that had their group assignment cleared remain visible and can be deleted by authorized users or admins.

**Why this priority**: Revocation should not break key management workflows or leave undeletable keys.

**Independent Test**: Revoke a group from a user and then delete a key that was unassigned during the update.

**Acceptance Scenarios**:

1. **Given** a key that became unassigned due to group revocation, **When** an authorized user or admin deletes the key, **Then** the deletion succeeds.

---

### User Story 3 - Bulk Group Updates Apply Consistently (Priority: P3)

Admins can update a user's allowed groups in one operation, and all keys tied to removed groups are unassigned as part of that update.

**Why this priority**: Admins need consistent results when multiple groups change at once.

**Independent Test**: Remove multiple groups in one update and verify every key bound to those groups is unassigned.

**Acceptance Scenarios**:

1. **Given** a user with keys bound to Group A and Group B, **When** an admin removes both groups in one update, **Then** all keys bound to those groups are unassigned and other keys remain unchanged.

---

### Edge Cases

- User loses all groups: all group-bound keys become unassigned and cannot access any group-scoped resources.
- Key already unassigned: it remains unassigned and unaffected by group updates.
- Group removed and later re-added: previously unassigned keys remain unassigned until explicitly re-bound.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: When a user's allowed groups are updated to remove one or more groups, the system MUST unassign those groups from any of the user's keys currently bound to them.
- **FR-002**: After a group is removed, any key previously bound to that group MUST NOT be usable to access that group's resources.
- **FR-003**: Group permission changes MUST take effect for authorization decisions within 60 seconds so revoked access cannot persist.
- **FR-004**: Keys with no group assignment MUST remain visible and deletable by authorized users or admins.
- **FR-005**: Updating allowed groups MUST NOT alter keys bound to groups that remain allowed.
- **FR-006**: A single allowed-groups update MUST handle multiple removed groups and unassign all related keys in that same update.

### Key Entities *(include if feature involves data)*

- **User**: An account with a set of allowed groups and ownership of API keys.
- **Group**: A scoped access domain that can be granted or revoked for a user.
- **API Key**: A credential owned by a user that may be assigned to a single group or to no group.

### Assumptions

- Unassigned keys have no group scope and require explicit re-binding before they can access a group again.
- Existing admin permissions to update allowed groups remain unchanged.

### Dependencies

- None.

### Out of Scope

- Changes to group definitions, billing, or subscription rules.
- Changes to key creation policies beyond clearing group assignments on revocation.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of keys bound to removed groups are unassigned within 60 seconds of the admin update.
- **SC-002**: 100% of authorization attempts using a key formerly bound to a removed group are denied within 60 seconds of the update.
- **SC-003**: 99.9% of deletion attempts for unassigned keys succeed.
- **SC-004**: 99% of multi-group updates result in all affected keys being unassigned within 60 seconds.
