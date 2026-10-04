import { SceneCanvas } from 'mokei/clay'
import { activateScene, registerScene } from 'mokei/scene'
import { useEffect } from 'react'
import { yardlineScene } from './scene'

export function YardCanvas({ play = true }: { play?: boolean }) {
  useEffect(() => {
    registerScene(yardlineScene)
    activateScene('yardline')
  }, [])

  return (
    <SceneCanvas quality="high" frameloop={play ? 'always' : 'never'} style={{ width: '100%', height: '100%' }}>
      <yardlineScene.World />
    </SceneCanvas>
  )
}

export default YardCanvas
