import { getUserKubeconfig, getUserKubeconfigNotPatch } from '@/services/backend/kubernetes/admin';
import { globalPrisma, prisma } from '@/services/backend/db/init';
import { getRegionUid } from '@/services/enable';
import { customAlphabet } from 'nanoid';
import { retrySerially } from '@/utils/tools';
import { AccessTokenPayload } from '@/types/token';
import { generateAccessToken, generateAppToken } from '@/services/backend/auth';
import { createNamespace } from '@/pages/api/auth/namespace/create';

const LetterBytes = 'abcdefghijklmnopqrstuvwxyz0123456789';
const HostnameLength = 8;

const nanoid = customAlphabet(LetterBytes, HostnameLength);

export async function get_k8s_username() {
  return await retrySerially<string | null>(async () => {
    const crName = nanoid();
    const result = await prisma.userCr.findUnique({
      where: {
        crName
      }
    });
    if (!result) return crName;
    else return Promise.reject(null);
  }, 3);
}

export async function getRegionToken({
  userUid,
  userId
}: {
  userUid: string;
  userId: string;
}): Promise<{
  kubeconfig: string;
  token: string;
  appToken: string;
}> {
  const region = await globalPrisma.region.findUnique({
    where: {
      uid: getRegionUid()
    }
  });
  if (!region) throw Error('The REGION_UID is undefined');

  let kcPatched = false;

  const payload = await retrySerially<AccessTokenPayload>(
    () =>
      prisma.$transaction(async (tx): Promise<AccessTokenPayload> => {
        let userCrResult = await tx.userCr.findUnique({
          where: {
            userUid
          },
          include: {
            userWorkspace: {
              include: {
                workspace: true
              }
            }
          }
        });
        if (!userCrResult) {
          const crName = nanoid();
          const userCrCreateResult = await tx.userCr.create({
            data: {
              crName,
              userUid
            }
          });
          if (!userCrCreateResult) {
            throw new Error('Failed to create userCr');
          }
          userCrResult = {
            ...userCrCreateResult,
            userWorkspace: []
          };
        }
        // get a exist user
        let workspaceId: string;
        let workspaceUid: string;
        if (userCrResult.userWorkspace.length === 0) {
          const kubeconfig = await getUserKubeconfig(userCrResult.uid, userCrResult.crName);
          if (!kubeconfig) {
            throw new Error('Failed to get user from k8s');
          }
          kcPatched = true;
          const relation = await createNamespace(
            'private team',
            userCrResult.uid,
            userCrResult.crName
          );
          if (!relation) {
            throw new Error('Failed to create namespace');
          }
          workspaceId = relation.id;
          workspaceUid = relation.uid;
        } else {
          workspaceId = userCrResult.userWorkspace[0].workspace.id;
          workspaceUid = userCrResult.userWorkspace[0].workspace.uid;
        }
        return {
          userUid: userCrResult.userUid,
          userCrUid: userCrResult.uid,
          userCrName: userCrResult.crName,
          regionUid: region.uid,
          userId,
          workspaceId,
          workspaceUid
        };
      }),
    3
  );

  if (!payload) {
    throw new Error('Failed to get user from db');
  }

  let kubeconfig;
  if (kcPatched) {
    kubeconfig = await getUserKubeconfigNotPatch(payload.userCrName);
  } else {
    kubeconfig = await getUserKubeconfig(payload.userCrUid, payload.userCrName);
  }
  if (!kubeconfig) {
    throw new Error('Failed to get user from k8s');
  }

  return {
    kubeconfig,
    token: generateAccessToken(payload),
    appToken: generateAppToken(payload)
  };
}
