import { SceneCanvas } from 'mokei/clay'
import { activateScene, registerScene } from 'mokei/scene'
import { useEffect } from 'react'
import { yardlineScene } from './scene'

export function YardCanvas() {
  useEffect(() => {
    registerScene(yardlineScene)
    activateScene('yardline')
  }, [])

  return (
    <SceneCanvas quality="high">
      <yardlineScene.World />
    </SceneCanvas>
  )
}

export default YardCanvas
