import CalculateOutlined from '@mui/icons-material/CalculateOutlined'
import DataObjectRounded from '@mui/icons-material/DataObjectRounded'
import PlaylistAddCheckRounded from '@mui/icons-material/PlaylistAddCheckRounded'
import RefreshRounded from '@mui/icons-material/RefreshRounded'
import { Alert, Box, Button, Checkbox, Chip, FormControl, IconButton, InputLabel, LinearProgress, MenuItem, Select, Stack, TextField, Tooltip, Typography } from '@mui/material'
import { useEffect, useMemo, useState } from 'react'
import { GeometryDetailDrawer } from '../components/common/GeometryDetailDrawer'
import { MapLegend } from '../components/common/MapLegend'
import { PageHeader } from '../components/common/PageHeader'
import { ResurveyPlanPanel } from '../components/common/ResurveyPlanPanel'
import { RunStateBadge } from '../components/common/RunStateBadge'
import { SurveyCanvas } from '../components/common/SurveyCanvas'
import { useAuth } from '../hooks/useAuth'
import { useCoverageLayers } from '../hooks/useCoverageLayers'
import { useCoverageGapStore } from '../stores/coverage-gap-store'
import { useSonarRunStore } from '../stores/sonar-run-store'
import { useSurveyAreaStore } from '../stores/survey-area-store'
import { GAP_SEVERITY_LABEL, GAP_STATE_LABEL, isResurveySelectable, type GapState } from '../types/enums/gap-severity'
import { geometryLineLength } from '../utils/geometry'
import { formatMetres, validateResurveySelection } from '../utils/resurvey'

const nextGapStates:Partial<Record<GapState,GapState[]>>={detected:['reviewed'],reviewed:['accepted','false_positive'],accepted:['resurveyed'],false_positive:['closed'],resurveyed:['closed']}

export function CoveragePage(){
  const {hasRole}=useAuth()
  const areas=useSurveyAreaStore(state=>state.items);const fetchAreas=useSurveyAreaStore(state=>state.fetch)
  const runs=useSonarRunStore(state=>state.items);const fetchRuns=useSonarRunStore(state=>state.fetch)
  const {items,selected,evidence,loading,fetch,select,detect,transition,checkedIds,toggleChecked,clearChecked,resurveyPlan,planError,planning,planResurvey}=useCoverageGapStore()
  const [areaID,setAreaID]=useState(1)
  const [runIDs,setRunIDs]=useState<number[]>([1])
  const [resolution,setResolution]=useState(20)
  const [drawer,setDrawer]=useState(false)
  const [note,setNote]=useState('已核对坐标系、输入校验和与补测线建议，结论仅用于离线规划。')

  useEffect(()=>{void Promise.all([fetch(),fetchAreas(),fetchRuns()])},[fetch,fetchAreas,fetchRuns])
  useEffect(()=>{if(areas[0]&&!areas.some(area=>area.id===areaID))setAreaID(areas[0].id)},[areas,areaID])
  useEffect(()=>{const eligible=runs.filter(run=>run.run_state==='processed'&&(run.transect_plan?.survey_area_id??0)===areaID).map(run=>run.id);if(eligible.length&&!runIDs.some(id=>eligible.includes(id)))setRunIDs([eligible[0]!])},[runs,areaID,runIDs])

  const area=selected?.survey_area??areas.find(value=>value.id===(selected?.survey_area_id??areaID))
  const sourceRuns=selected?runs.filter(run=>selected.source_run_ids.includes(run.id)):runs.filter(run=>runIDs.includes(run.id))
  const plan=sourceRuns[0]?.transect_plan
  const layers=useCoverageLayers(area,plan,sourceRuns,selected)
  const legends=useMemo(()=>layers.map(layer=>({label:layer.label,color:layer.color,pattern:layer.dash?'dash' as const:layer.fill?'fill' as const:'solid' as const})),[layers])

  const checkedGaps=useMemo(()=>checkedIds.map(id=>items.find(item=>item.id===id)).filter((item):item is NonNullable<typeof item>=>Boolean(item)),[checkedIds,items])
  const selectionIssue=useMemo(()=>validateResurveySelection(checkedGaps),[checkedGaps])
  const checkedAreaCode=checkedGaps[0]?checkedGaps[0].survey_area?.area_code??`测区 #${checkedGaps[0].survey_area_id}`:''

  const calculate=async()=>detect({survey_area_id:areaID,source_run_ids:runIDs,algorithm_version:'grid-cover-v1.0.0',resolution_m:resolution})
  const generateResurveyPlan=async()=>{
    if(selectionIssue)return
    await planResurvey(checkedGaps.map(gap=>gap.id))
  }
  const eligibleRuns=runs.filter(run=>run.run_state==='processed'&&(run.transect_plan?.survey_area_id??0)===areaID)
  const canCalculate=hasRole('admin','data_processor')
  const canReview=hasRole('reviewer')
  const canPlanResurvey=hasRole('admin','reviewer')

  return <>
    <PageHeader eyebrow="COVERAGE EVIDENCE REVIEW" title="覆盖分析" description="冻结测区与已处理航迹，计算覆盖、重复扫测和疑似缺口；多快照补测调度仅生成只读任务单，不改变原快照状态。" actions={<Tooltip title="刷新"><IconButton aria-label="刷新覆盖结果" onClick={()=>void Promise.all([fetch(),fetchRuns()])}><RefreshRounded/></IconButton></Tooltip>}/>
    {loading&&<LinearProgress/>}
    <Box className="coverage-controls">
      <Box><Typography variant="overline">NEW ANALYSIS</Typography><Typography variant="h6">冻结本次输入</Typography></Box>
      <FormControl size="small" sx={{minWidth:180}}><InputLabel>测区</InputLabel><Select label="测区" value={areaID} onChange={event=>{setAreaID(Number(event.target.value));setRunIDs([])}}>{areas.map(area=><MenuItem value={area.id} key={area.id}>{area.area_code}</MenuItem>)}</Select></FormControl>
      <FormControl size="small" sx={{minWidth:220}}><InputLabel>已处理运行</InputLabel><Select multiple label="已处理运行" value={runIDs} onChange={event=>setRunIDs((event.target.value as number[]))}>{eligibleRuns.map(run=><MenuItem value={run.id} key={run.id}>{run.run_code}</MenuItem>)}</Select></FormControl>
      <TextField size="small" type="number" label="网格 (m)" value={resolution} onChange={event=>setResolution(Number(event.target.value))} sx={{width:120}}/>
      {canCalculate&&<Button variant="contained" startIcon={<CalculateOutlined/>} disabled={runIDs.length===0} onClick={()=>void calculate()}>计算覆盖</Button>}
    </Box>

    <Box className="resurvey-controls">
      <Box>
        <Typography variant="overline">MULTI-SNAPSHOT RESURVEY</Typography>
        <Typography variant="h6">多快照补测调度</Typography>
        <Typography variant="caption" color="text.secondary">在左侧历史快照勾选同一测区 2 条以上「待复核」或「已复核」快照；含其他测区、已关闭或重复快照时整次拒绝。</Typography>
      </Box>
      {checkedIds.length>0&&<Chip size="small" label={`已勾选 ${checkedIds.length} 条 · ${checkedAreaCode}`} onDelete={clearChecked} variant="outlined"/>}
      {canPlanResurvey&&<Tooltip title={selectionIssue?.message??'按严重度、缺口面积、发现时间生成确定性任务顺序'}>
        <span>
          <Button variant="contained" color="secondary" startIcon={<PlaylistAddCheckRounded/>} loading={planning}
            disabled={checkedIds.length<2||Boolean(selectionIssue)}
            onClick={()=>void generateResurveyPlan()}>生成补测任务单</Button>
        </span>
      </Tooltip>}
    </Box>
    {checkedIds.length>=2&&selectionIssue&&<Alert severity="warning" sx={{mb:2}}>{selectionIssue.message}</Alert>}
    {planError&&<Alert severity="error" sx={{mb:2}} onClose={()=>useCoverageGapStore.setState({planError:null})}>{planError}（整次拒绝，原快照保持不变）</Alert>}
    {resurveyPlan&&<Box sx={{mb:3}}><ResurveyPlanPanel plan={resurveyPlan}/></Box>}

    <Box className="coverage-workspace">
      <Box className="result-rail">
        <Box className="section-heading"><Typography variant="h6">历史快照</Typography><Typography variant="caption">{items.length} 条</Typography></Box>
        {items.map(item=>{
          const checked=checkedIds.includes(item.id)
          const selectable=isResurveySelectable(item.gap_state)
          return (
            <Box key={item.id} className={selected?.id===item.id?'gap-row active':'gap-row'} sx={{pr:0.5}}>
              {canPlanResurvey
                ? <Tooltip title={selectable?'加入补测调度':'仅待复核或已复核快照可勾选'}>
                    <Checkbox size="small" checked={checked} disabled={!selectable&&!checked} onChange={()=>toggleChecked(item.id)} inputProps={{'aria-label':`勾选快照 #${item.id}`}}/>
                  </Tooltip>
                : <Box sx={{width:10}}/>}
              <Box component="button" type="button" className="gap-row-button" onClick={()=>select(item)}>
                <span><strong>#{item.id} · {GAP_SEVERITY_LABEL[item.severity]}</strong><small>{item.survey_area?.area_code??`AREA #${item.survey_area_id}`} · {item.algorithm_version}</small></span>
                <span><Chip size="small" label={GAP_STATE_LABEL[item.gap_state]} variant="outlined"/><small>缺口 {(item.gap_ratio*100).toFixed(1)}%</small></span>
              </Box>
            </Box>
          )
        })}
      </Box>

      <Box className="coverage-map">
        <Box className="section-heading">
          <Box><Typography variant="overline">SWATH / GAP / RESURVEY</Typography><Typography variant="h6">{selected?`缺口快照 #${selected.id}`:'等待覆盖计算'}</Typography></Box>
          <Tooltip title="查看缺口 GeoJSON"><span><IconButton aria-label="查看缺口 GeoJSON" disabled={!selected} onClick={()=>setDrawer(true)}><DataObjectRounded/></IconButton></span></Tooltip>
        </Box>
        <SurveyCanvas layers={layers}/><MapLegend items={legends}/>
        {selected&&<>
          <Box className="coverage-score">
            <div><span>覆盖率</span><strong>{(selected.coverage_ratio*100).toFixed(1)}%</strong></div>
            <div><span>重叠率</span><strong>{(selected.overlap_ratio*100).toFixed(1)}%</strong></div>
            <div><span>缺口率</span><strong>{(selected.gap_ratio*100).toFixed(1)}%</strong></div>
            <div><span>处理耗时</span><strong>{selected.processing_millis} ms</strong></div>
          </Box>
          <Alert severity={selected.severity==='critical'?'error':'warning'} sx={{mt:2}}>{selected.explanation}</Alert>
          {evidence&&<Typography variant="caption" color="text.secondary" display="block" sx={{mt:1}}>{evidence.decision_boundary_note}</Typography>}
        </>}
      </Box>

      <Box className="review-pane">
        <Typography variant="overline">HUMAN DECISION</Typography>
        <Typography variant="h6">人工复核</Typography>
        {selected?<>
          <Box component="dl" className="evidence-list">
            <dt>状态</dt><dd>{GAP_STATE_LABEL[selected.gap_state]}</dd>
            <dt>严重度</dt><dd>{GAP_SEVERITY_LABEL[selected.severity]}</dd>
            <dt>缺口面积</dt><dd>{selected.area_square_m.toFixed(1)} m²</dd>
            <dt>建议线长度</dt><dd>{formatMetres(geometryLineLength(selected.recommended_line_geojson))}</dd>
            <dt>算法版本</dt><dd>{selected.algorithm_version}</dd>
            <dt>输入哈希</dt><dd className="hash">{selected.input_hash}</dd>
            <dt>发现时间</dt><dd>{new Date(selected.detected_at).toLocaleString('zh-CN',{hour12:false})}</dd>
            <dt>运行来源</dt><dd>{selected.source_run_ids.map(id=><RunStateBadge key={id} state={runs.find(run=>run.id===id)?.run_state??'processed'}/>)}</dd>
          </Box>
          {canReview&&(nextGapStates[selected.gap_state]?.length??0)>0&&<>
            <TextField multiline minRows={3} fullWidth label="复核意见" value={note} onChange={event=>setNote(event.target.value)} sx={{mt:2}}/>
            <Stack gap={1} sx={{mt:1.5}}>{nextGapStates[selected.gap_state]?.map(target=><Button key={target} variant={target==='false_positive'?'outlined':'contained'} onClick={()=>void transition(selected,target,note)}>{GAP_STATE_LABEL[target]}</Button>)}</Stack>
          </>}
        </>:<Typography color="text.secondary" sx={{mt:1}}>运行一次覆盖计算后，可查看冻结哈希和人工决策状态。</Typography>}
      </Box>
    </Box>

    <GeometryDetailDrawer open={drawer} onClose={()=>setDrawer(false)} title={selected?`覆盖缺口 #${selected.id}`:'覆盖缺口'} geometry={selected?.gap_geojson} metadata={selected?{'严重度':GAP_SEVERITY_LABEL[selected.severity],'覆盖率':`${(selected.coverage_ratio*100).toFixed(2)}%`,'缺口面积':`${selected.area_square_m.toFixed(1)} m²`,'算法版本':selected.algorithm_version,'输入哈希':selected.input_hash}:undefined}/>
  </>
}
