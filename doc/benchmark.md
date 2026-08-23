测量：串行请求 ×10 取平均；原站使用与客户端一致的真实登录凭证;
`list_comments` 统一取参数 `page=1&limit=10&comment_limit=10`

| endpoints | local avg | treehole avg | accelerate |
|---|---|---|---|
| `list_comments` | 3.7 ms | 279 ms | 76× |
| `…&keyword=zkc` | 4.6 ms | 774 ms | 169× |
| `…&keyword=zkc&kind=1` | 7.3 ms | **27,494 ms** | **3,743×** |
| `…&keyword=zkc&label=1` | 9.2 ms | **31,862 ms** | **3,482×** |
| `…&kind=1` | 3.9 ms | 2,725 ms | 694× |
| `…&label=1` | 2.9 ms | 611 ms | 210× |
| `hole/one?pid=8450153` | 2.4 ms | 113 ms | 47× |
| `comment/list?pid=8450153&page=1&limit=10` | 3.2 ms | 94 ms | 29× |

备注: 原站存在固定网络开销约 50 ms，本地使用localhost loopback 无此成本，但在优化关键路径上可以忽略;

