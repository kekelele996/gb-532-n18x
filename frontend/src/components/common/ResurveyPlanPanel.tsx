import AssignmentLateRounded from '@mui/icons-material/AssignmentLateRounded'
import { Alert, Box, Chip, Paper, Stack, Table, TableBody, TableCell, TableHead, TableRow, Tooltip, Typography } from '@mui/material'
import type { ResurveyPlan } from '../../types/coverage-gap'
import { GAP_SEVERITY_LABEL, GAP_STATE_LABEL } from '../../types/enums/gap-severity'
import { formatMetres, formatSquareMetres } from '../../utils/resurvey'

const severityColor = { critical: 'error', major: 'warning', minor: 'default' } as const

export function ResurveyPlanPanel({ plan }: { plan: ResurveyPlan }) {
  return (
    <Paper className="resurvey-plan" variant="outlined">
      <Box className="section-heading">
        <Box>
          <Typography variant="overline">MULTI-SNAPSHOT RESURVEY ORDER</Typography>
          <Typography variant="h6"><AssignmentLateRounded fontSize="small" sx={{ verticalAlign: -3, mr: 0.5 }} />补测任务单 · {plan.area_code || `测区 #${plan.survey_area_id}`}</Typography>
        </Box>
        <Stack gap={1} direction="row" flexWrap="wrap" justifyContent="flex-end">
          <Chip size="small" label={`任务 ${plan.task_count} 条`} variant="outlined" />
          <Chip size="small" color="warning" variant="outlined" label={`汇总缺口面积 ${formatSquareMetres(plan.total_gap_area_square_m)}`} />
          <Chip size="small" color="secondary" variant="outlined" label={`建议线合计 ${formatMetres(plan.total_line_length_m)}`} />
          <Tooltip title="相同快照集合重复生成时指纹一致，可用于核对结果确定性">
            <Chip size="small" label={`任务单指纹 ${plan.plan_fingerprint.slice(0, 12)}`} variant="outlined" />
          </Tooltip>
        </Stack>
      </Box>
      <Alert severity="info" variant="outlined" sx={{ borderRadius: 0 }}>{plan.sorting_policy}</Alert>
      <Box className="data-table-wrap">
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell width={52}>顺序</TableCell>
              <TableCell width={92}>快照</TableCell>
              <TableCell width={84}>严重度</TableCell>
              <TableCell width={110}>缺口面积</TableCell>
              <TableCell width={120}>建议线长度</TableCell>
              <TableCell width={92}>当前状态</TableCell>
              <TableCell width={150}>发现时间</TableCell>
              <TableCell>排序理由</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {plan.tasks.map(task => (
              <TableRow key={task.gap_id}>
                <TableCell><strong>{task.order}</strong></TableCell>
                <TableCell>#{task.gap_id}</TableCell>
                <TableCell><Chip size="small" color={severityColor[task.severity]} label={GAP_SEVERITY_LABEL[task.severity]} variant="outlined" /></TableCell>
                <TableCell>{formatSquareMetres(task.area_square_m)}</TableCell>
                <TableCell>{formatMetres(task.recommended_line_m)}</TableCell>
                <TableCell>{GAP_STATE_LABEL[task.gap_state]}</TableCell>
                <TableCell><Typography variant="caption">{new Date(task.detected_at).toLocaleString('zh-CN', { hour12: false })}</Typography></TableCell>
                <TableCell><Typography variant="body2">{task.sort_rationale}</Typography></TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </Box>
    </Paper>
  )
}
