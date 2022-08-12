# TinyExcel2Json


## 游戏策划导表工具
仅能够将excel 转成 json, 无其他功能. 

- 支持嵌套消息和数组, 扩展方便
- 每个嵌套消息或者数组都分拆到各列, 方便策划拉表设计数值.
- 无环境依赖, 一个exe即所有.

## 使用说明
- sheet表名需要设置为Config结尾, 转换为sheet表名.json
- 表结构支持无限层嵌套, 但是不建议使用超过两层, 以防止被同事劈. 
- 有限支持空值. 限制只能是array中子消息/子字段可以为空;比如奖励, 有的配5个物品, 有的配1个物品, 这个时候可以有多个为空.

## 例子
直接看例子 examples


## TODO 
- 多key作为id
- ~~id关联检查~~ (程序使用配置会做关联检查)