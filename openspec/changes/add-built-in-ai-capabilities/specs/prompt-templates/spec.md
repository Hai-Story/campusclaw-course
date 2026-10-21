## Purpose

为教师提供可预览和复用的角色提示词模板，预置教学模板并允许维护班级自定义模板，通过复制正文配置助手，减少重复输入并避免模板更新意外改变已有助手行为。

## ADDED Requirements

### Requirement: Class template lifecycle
系统 SHALL 提供三个只读内置模板与本班自定义模板，教师可列表、创建、修改和删除自定义模板；学生访问返回 403，跨班与不存在编号返回同形 404。名称为 1–64 字，prompt 为 1–2000 字，班内名称唯一，重名返回 400。

#### Scenario: Protected templates
- **WHEN** 教师修改内置模板或学生访问模板接口
- **THEN** 返回 403，数据保持不变

#### Scenario: Custom template lifecycle
- **WHEN** 教师创建、编辑、复制或删除本班模板
- **THEN** 操作只影响本班，其他班级不能读取或修改，复制内置模板产生可编辑副本

### Requirement: Apply a template snapshot
教师 SHALL 能选择、预览并套用模板，替换非空角色提示词前须确认。保存助手后提示词生效，后续模板编辑或删除不得改写已保存助手。

#### Scenario: Independent saved assistant
- **WHEN** 教师套用模板保存助手，再修改或删除模板
- **THEN** 助手仍保留套用时的提示词正文
