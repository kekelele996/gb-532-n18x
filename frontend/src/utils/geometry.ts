import type { GeoJSONFeature, Position } from '../types/api'

export function geometryLines(feature:GeoJSONFeature):Position[][]{
  const geometry=feature.geometry
  if(geometry.type==='LineString')return[geometry.coordinates]
  if(geometry.type==='MultiLineString')return geometry.coordinates
  if(geometry.type==='Polygon')return geometry.coordinates
  return geometry.coordinates.flat()
}

export function geometryBounds(features:GeoJSONFeature[]){
  const points=features.flatMap(feature=>geometryLines(feature).flat())
  if(points.length===0)return null
  const xs=points.map(point=>point[0]),ys=points.map(point=>point[1])
  return{minX:Math.min(...xs),minY:Math.min(...ys),maxX:Math.max(...xs),maxY:Math.max(...ys),pointCount:points.length}
}

/** 累加 LineString/MultiLineString 各段平面距离（输入必须为米制投影坐标）。 */
export function geometryLineLength(feature:GeoJSONFeature):number{
  if(feature.geometry.type!=='LineString'&&feature.geometry.type!=='MultiLineString')return 0
  const lines:Position[][]=feature.geometry.type==='LineString'?[feature.geometry.coordinates]:feature.geometry.coordinates
  let total=0
  for(const line of lines){
    for(let index=1;index<line.length;index++){
      const [x1,y1]=line[index-1]!;const [x2,y2]=line[index]!
      total+=Math.hypot(x2-x1,y2-y1)
    }
  }
  return total
}
