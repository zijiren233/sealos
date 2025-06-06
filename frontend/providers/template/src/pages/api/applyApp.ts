import { authSession } from '@/services/backend/auth';
import { getK8s } from '@/services/backend/kubernetes';
import { handleK8sError, jsonRes } from '@/services/backend/response';
import type { NextApiRequest, NextApiResponse } from 'next';

export default async function handler(req: NextApiRequest, res: NextApiResponse) {
  const { yamlList, type = 'create' } = req.body as {
    yamlList: string[];
    type: 'create' | 'replace' | 'dryrun';
  };

  if (!yamlList) {
    return jsonRes(res, {
      code: 500,
      error: 'yaml list is empty'
    });
  }

  try {
    const { applyYamlList } = await getK8s({
      kubeconfig: await authSession(req.headers)
    });

    const applyRes = await applyYamlList(yamlList, type);

    jsonRes(res, { data: applyRes.map((item) => item.kind) });
  } catch (err: any) {
    return jsonRes(res, handleK8sError(err));
  }
}
