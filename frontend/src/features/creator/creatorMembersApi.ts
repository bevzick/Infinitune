import { apiRequest } from "../../shared/api/client";

export type CreatorMemberRole = "owner" | "manager" | "editor" | "uploader";

export type CreatorMember = {
  user_id: string;
  role: CreatorMemberRole;
  created_at: string;
};

export type ManagedMemberRole = Exclude<CreatorMemberRole, "owner">;

export function listCreatorMembers(
  accessToken: string,
  creatorId: string,
  signal?: AbortSignal,
) {
  return apiRequest<CreatorMember[]>(`/v1/creators/${creatorId}/members`, {
    accessToken,
    signal,
  });
}

export function addCreatorMember(
  accessToken: string,
  creatorId: string,
  input: { user_id: string; role: ManagedMemberRole },
) {
  return apiRequest<CreatorMember>(`/v1/creators/${creatorId}/members`, {
    accessToken,
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function changeCreatorMemberRole(
  accessToken: string,
  creatorId: string,
  userId: string,
  role: ManagedMemberRole,
) {
  return apiRequest<CreatorMember>(
    `/v1/creators/${creatorId}/members/${userId}`,
    {
      accessToken,
      method: "PATCH",
      body: JSON.stringify({ role }),
    },
  );
}

export function removeCreatorMember(
  accessToken: string,
  creatorId: string,
  userId: string,
) {
  return apiRequest<void>(`/v1/creators/${creatorId}/members/${userId}`, {
    accessToken,
    method: "DELETE",
  });
}
