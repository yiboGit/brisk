## 注意
通用api
- 微信相关
- 图片合成
### 图片合并 请求参数
```json
{
 "base":"https://epj-images.oss-cn-shanghai.aliyuncs.com/activities/gift1.jpg",
 "subImages":[
  { 
   "url":"https://epj-images.oss-cn-shanghai.aliyuncs.com/weixin-head/o-SEIwQ-6TyHbEMc0m9CZzZzlzLM.jpg",
   "Left":{"relative":false,"value":50},
   "Top":{"relative":false,"value":50},
   "width":128,
   "height":128,
   "withArc":true},
  {
   "url":"https://epj-images.oss-cn-shanghai.aliyuncs.com/weixin-head/o-SEIwQ-6TyHbEMc0m9CZzZzlzLM.jpg",
   "left":{"relative":false,"value":0},
   "top":{"relative":true,"value":0},
   "width":128,
   "height":128,
   "withArc":true},
  {
   "url":"https://epj-images.oss-cn-shanghai.aliyuncs.com/weixin-head/o-SEIwQ-6TyHbEMc0m9CZzZzlzLM.jpg",
   "left":{"relative":false,"value":200},
   "top":{"relative":false,"value":200},
   "width":128,
   "height":128,
   "withArc":true}
  ],
 "text":{
   "content":"小天",
   "left": {"relative":false,"value":0},
   "top":{"relative":false,"value":0}
   },
 "extra":{"width":0,"height":0,"AddWidth":0,"AddHeight":256}}
```