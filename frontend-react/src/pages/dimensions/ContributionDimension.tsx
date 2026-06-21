// 「贡献」维度内容（4 主体共用，壳内 <Outlet> 渲染）。按 entity 分支到 4 个独立贡献页：
//   org → OrgContribution（部门交付物：合并需求/代码行/提交，看板派生·dept-tree/ranking）
//   user → UserContribution（个人交付物：合并需求/代码行/提交，看板派生·getAllUsersV2）
//   project → ProjectContribution（项目交付物：Need/生成代码/贡献者，看板派生·projectList）
//   repo → RepoContribution（仓库交付物：提交/代码行/分支/贡献者，看板派生·repos/detail）
// 口径决策⑤：贡献全程用看板派生（提交/代码行/合并需求）；平台 tokens=消耗量≠贡献，故本维度零平台请求。
// 每个子页自取 useEntityFocus() 的聚焦对象，并自管聚合/聚焦两态（与其它维度组件同构）。
import { useEntityFocus } from '@/components/layout/matrix'
import OrgContribution from '@/pages/dimensions/contrib/OrgContribution'
import UserContribution from '@/pages/dimensions/contrib/UserContribution'
import ProjectContribution from '@/pages/dimensions/contrib/ProjectContribution'
import RepoContribution from '@/pages/dimensions/contrib/RepoContribution'

export default function ContributionDimension() {
  const { entity } = useEntityFocus()

  switch (entity) {
    case 'org':
      return <OrgContribution />
    case 'project':
      return <ProjectContribution />
    case 'repo':
      return <RepoContribution />
    case 'user':
    default:
      return <UserContribution />
  }
}
