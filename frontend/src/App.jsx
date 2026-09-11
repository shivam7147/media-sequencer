import { BrowserRouter, Route, Routes } from 'react-router-dom'
import Wall from './pages/Wall'
import SingleWindow from './pages/SingleWindow'

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Wall />} />
        <Route path="/window/:id" element={<SingleWindow />} />
      </Routes>
    </BrowserRouter>
  )
}
