// 路由/聚焦切换时滚到页顶。挂在 AppShell 内（router 树内），不渲染任何 DOM。
// 触发：
//   ① pathname 变化——切维度 Tab / 进详情叶子（/user/:id 等）/ 切主体（始终滚顶）。
//   ② 聚焦对象变化（?object=<id>，pathname 不变）——从排行点一个对象进聚焦后从顶开始，否则停在表中间。
// 例外（修 #7）：组织页（/org）的 OrgTree 是「树内就地导航」复合视图，点部门节点会改 object，
//   此时不该强制滚顶（否则在树里点来点去每次都跳回顶部，很烦）。故 /org 只在 pathname 变时滚顶，
//   其它主体保留「排行点行→聚焦从顶开始」。
import { useEffect } from 'react'
import { useLocation, useSearchParams } from 'react-router'

export default function ScrollToTop() {
  const { pathname } = useLocation()
  const [searchParams] = useSearchParams()
  const object = searchParams.get('object') || ''

  // org 页：滚顶只跟 pathname；其它主体：pathname 或 object 变都滚顶。
  const scrollKey = pathname.startsWith('/org') ? pathname : `${pathname}?object=${object}`

  useEffect(() => {
    window.scrollTo(0, 0)
  }, [scrollKey])

  return null
}
