import { getTeamKubeconfig } from '@/services/backend/kubernetes/admin';
import { GetUserDefaultNameSpace } from '@/services/backend/kubernetes/user';
import { jsonRes } from '@/services/backend/response';
import { bindingRole, modifyWorkspaceRole } from '@/services/backend/team';
import { getTeamLimit } from '@/services/enable';
import { NamespaceDto, UserRole } from '@/types/team';
import { NextApiRequest, NextApiResponse } from 'next';
import { prisma } from '@/services/backend/db/init';
import { get_k8s_username } from '@/services/backend/regionAuth';
import { verifyAccessToken } from '@/services/backend/auth';
import { cache } from 'react';

const TEAM_LIMIT = getTeamLimit();

// Function to create namespace
export async function createNamespace(
  displayName: string,
  userCrUid: string,
  owner: string,
  namespace?: string
): Promise<NamespaceDto | null> {
  const workspace_creater = namespace || (await get_k8s_username());
  if (!workspace_creater) throw new Error('fail to get workspace_creater');
  // ns-
  const workspaceId = GetUserDefaultNameSpace(workspace_creater);

  // 创建伪user
  const creater_kc_str = await getTeamKubeconfig(workspace_creater, owner);
  if (!creater_kc_str) throw new Error('fail to get kubeconfig');

  const workspace = await prisma.workspace.create({
    data: {
      id: workspaceId,
      displayName
    }
  });

  if (!workspace) throw new Error(`failed to create namespace: ${workspaceId}`);

  // 分配owner权限
  const utnResult = await bindingRole({
    userCrUid: userCrUid,
    ns_uid: workspace.uid,
    role: UserRole.Owner,
    direct: true
  });

  if (!utnResult) throw new Error(`fail to binding namesapce: ${workspace.id}`);

  await modifyWorkspaceRole({
    role: UserRole.Owner,
    action: 'Create',
    workspaceId,
    k8s_username: owner
  });

  return {
    role: UserRole.Owner,
    createTime: workspace.createdAt,
    uid: workspace.uid,
    id: workspace.id,
    teamName: workspace.displayName
  };
}

export default async function handler(req: NextApiRequest, res: NextApiResponse) {
  try {
    const payload = await verifyAccessToken(req.headers);
    if (!payload) return jsonRes(res, { code: 401, message: 'token verify error' });

    const { teamName } = req.body as { teamName?: string };
    if (!teamName) return jsonRes(res, { code: 400, message: 'teamName is required' });

    const currentNamespaces = await prisma.userWorkspace.findMany({
      where: {
        userCrUid: payload.userCrUid,
        status: 'IN_WORKSPACE'
      },
      include: {
        workspace: {
          select: {
            displayName: true
          }
        }
      }
    });

    if (currentNamespaces.length >= TEAM_LIMIT)
      return jsonRes(res, { code: 403, message: 'The number of teams created is too many' });

    const alreadyNamespace = currentNamespaces.findIndex(
      (utn) => utn.workspace.displayName === teamName
    );
    if (alreadyNamespace > -1)
      return jsonRes(res, { code: 409, message: 'The team is already exist' });

    const user = await prisma.userCr.findUnique({
      where: {
        userUid: payload.userUid,
        uid: payload.userCrUid
      }
    });

    if (!user) throw new Error('fail to get user');

    const namespace = await createNamespace(teamName, user.uid, payload.userCrName);

    if (namespace) {
      jsonRes<{ namespace: NamespaceDto }>(res, {
        code: 200,
        message: 'Successfully',
        data: { namespace }
      });
    } else {
      jsonRes(res, { code: 500, message: 'failed to create team workspace' });
    }
  } catch (e) {
    console.log(e);
    jsonRes(res, { code: 500, message: 'failed to create team workspace' });
  }
}
