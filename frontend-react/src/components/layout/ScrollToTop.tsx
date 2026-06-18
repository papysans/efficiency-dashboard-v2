// 路由/聚焦切换时滚到页顶。挂在 AppShell 内（router 树内），不渲染任何 DOM。
// 两类触发：
//   ① pathname 变化——切维度 Tab / 进详情叶子（/user/:id 等）/ 切主体。
//   ② 聚焦对象变化——聚焦态只改 ?object=<id>（pathname 不变），从排行点一个对象进聚焦后
//      也要从顶部开始，否则会停在排行表中间。故额外监听 search 里的 object 参数。
// 只在导航/聚焦切换时滚顶，不干扰页内正常滚动。
import { useEffect } from 'react'
import { useLocation, useSearchParams } from 'react-router'

export default function ScrollToTop() {
  const { pathname } = useLocation()
  const [searchParams] = useSearchParams()
  const object = searchParams.get('object') || ''

  useEffect(() => {
    window.scrollTo(0, 0)
  }, [pathname, object])

  return null
}
